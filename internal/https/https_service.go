package https

import (
	"bufio"
	"bytes"
	crand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	mrand "math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Snikimonkd/mitm_proxy/internal/models"
	"github.com/Snikimonkd/mitm_proxy/internal/proxy"
	"github.com/sirupsen/logrus"
)

type HttpsService struct {
	proxyWriter             http.ResponseWriter
	interceptedHttpsRequest *http.Request
	HttpsRequest            *http.Request
	scheme                  string
	config                  *tls.Config
	crutch                  proxy.ProxyRepository
}

func NewHttpsService(writer http.ResponseWriter, interceptedHttpsRequest *http.Request, proxyRepo proxy.ProxyRepository) *HttpsService {
	requestedUrl, _ := url.Parse(interceptedHttpsRequest.RequestURI)
	var scheme string
	if requestedUrl.Scheme == "" {
		scheme = interceptedHttpsRequest.URL.Host
	} else {
		scheme = requestedUrl.Scheme
	}

	rootDir, _ := os.Getwd()
	certsPath := rootDir + "/genCerts/certs"

	_, errStat := os.Stat(certsPath)
	if os.IsNotExist(errStat) {
		err := os.MkdirAll(certsPath, 0700)
		if err != nil {
			logrus.Error(err)
		}
	}

	return &HttpsService{
		proxyWriter:             writer,
		interceptedHttpsRequest: interceptedHttpsRequest,
		scheme:                  scheme,
		crutch:                  proxyRepo,
	}
}

func (hs *HttpsService) ProxyHttpsRequest() error {
	hijackedConn, err := hs.interceptConnection()
	if err != nil {
		return err
	}

	TCPClientConn, err := hs.initializeTCPClient(hijackedConn)
	if err != nil {
		return err
	}

	TCPServerConn, err := hs.initializeTCPServer()
	if err != nil {
		return err
	}

	err = hs.doHttpsRequest(TCPClientConn, TCPServerConn)
	if err != nil {
		return err
	}

	defer hijackedConn.Close()
	return nil
}

func (hs *HttpsService) SaveReqToDB(request *http.Request) error {
	requestBody := new(bytes.Buffer)
	_, err := io.Copy(requestBody, request.Body)
	if err != nil {
		return err
	}

	req := &models.Request{
		Method:  request.Method,
		Scheme:  "https",
		Host:    request.Host,
		Path:    request.URL.Path,
		Headers: request.Header,
		Body:    requestBody.String(),
	}

	err = hs.crutch.InsertInto(req)
	if err != nil {
		return err
	}

	logrus.Info("RequestId: ", req.Id)
	hs.HttpsRequest.Body = ioutil.NopCloser(strings.NewReader(req.Body))

	return nil
}

func (hs *HttpsService) generateCertificate() (tls.Certificate, error) {
	rootDir, err := os.Getwd()
	if err != nil {
		return tls.Certificate{}, err
	}

	cmdGenDir := rootDir + "/genCerts"
	certsDir := cmdGenDir + "/certs/"

	certFilename := certsDir + hs.scheme + ".crt"

	_, errStat := os.Stat(certFilename)
	if os.IsNotExist(errStat) {
		genCommand := exec.Command(cmdGenDir+"/gen_cert.sh", hs.scheme, strconv.Itoa(mrand.Intn(100000000)))

		_, err := genCommand.CombinedOutput()
		if err != nil {
			return tls.Certificate{}, err
		}
	}

	cert, err := tls.LoadX509KeyPair(certFilename, cmdGenDir+"/cert.key")
	if err != nil {
		return tls.Certificate{}, err
	}

	return cert, nil
}

func (hs *HttpsService) interceptConnection() (net.Conn, error) {
	hijacker, ok := hs.proxyWriter.(http.Hijacker)
	if !ok {
		return nil, errors.New("creating hijacker failed")
	}

	hijackedConn, _, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	_, err = hijackedConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))

	if err != nil {
		hijackedConn.Close()
		return nil, err
	}
	return hijackedConn, nil
}

func (hs *HttpsService) initializeTCPClient(hijackedConn net.Conn) (*tls.Conn, error) {
	cert, err := hs.generateCertificate()
	if err != nil {
		return nil, err
	}

	hs.config = &tls.Config{Certificates: []tls.Certificate{cert}, ServerName: hs.scheme}

	TCPClientConn := tls.Server(hijackedConn, hs.config)
	err = TCPClientConn.Handshake()
	if err != nil {
		TCPClientConn.Close()
		hijackedConn.Close()
		return nil, err
	}

	clientReader := bufio.NewReader(TCPClientConn)
	TCPClientRequest, err := http.ReadRequest(clientReader)
	if err != nil {
		return nil, err
	}

	hs.HttpsRequest = TCPClientRequest
	hs.SaveReqToDB(TCPClientRequest)

	return TCPClientConn, nil
}

func (hs *HttpsService) initializeTCPServer() (*tls.Conn, error) {
	file, err := os.Open("./genCerts/ca.key")
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	privPem, _ := pem.Decode(b)

	priv, err := x509.ParsePKCS1PrivateKey(privPem.Bytes)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{hs.scheme},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 180),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(crand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}

	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})
	cert, err := tls.X509KeyPair(caPEM.Bytes(), b)
	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	TCPServerConn, err := tls.Dial("tcp", hs.interceptedHttpsRequest.Host, conf)
	if err != nil {
		return nil, err
	}

	return TCPServerConn, nil
}

func (hs *HttpsService) transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func (hs *HttpsService) doHttpsRequest(TCPClientConn *tls.Conn, TCPServerConn *tls.Conn) error {
	err := hs.HttpsRequest.Write(TCPServerConn)
	if err != nil {
		return err
	}

	serverReader := bufio.NewReader(TCPServerConn)
	TCPServerResponse, err := http.ReadResponse(serverReader, hs.HttpsRequest)
	if err != nil {
		return err
	}

	rawResp, err := httputil.DumpResponse(TCPServerResponse, true)
	_, err = TCPClientConn.Write(rawResp)
	if err != nil {
		return err
	}
	go hs.transfer(TCPClientConn, TCPServerConn)
	go hs.transfer(TCPServerConn, TCPClientConn)

	defer TCPClientConn.Close()
	return nil
}
