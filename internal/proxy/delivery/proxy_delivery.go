package delivery

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/Snikimonkd/mitm_proxy/internal/proxy"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type ProxyDelivery struct {
	needSave   bool
	proxyUcase proxy.ProxyUsecase
}

func NewProxyDelivery(proxyUsecase proxy.ProxyUsecase) *ProxyDelivery {
	return &ProxyDelivery{
		proxyUcase: proxyUsecase,
		needSave:   true,
	}
}

func (pd *ProxyDelivery) HandleRequest(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodConnect {
		err := pd.proxyUcase.HandleHttpsRequest(writer, request, pd.needSave)
		if err != nil {
			logrus.Error(err)
		}
	} else {
		if pd.needSave {
			err := pd.proxyUcase.SaveReqToDB(request, "http")
			if err != nil {
				logrus.Error(err)
			}
		}

		_, err := pd.proxyUcase.HandleHttpRequest(writer, request)
		if err != nil {
			logrus.Info(err)
		}
	}
	pd.needSave = true
}

func (pd *ProxyDelivery) GetAllRequestsHandler(writer http.ResponseWriter, request *http.Request) {
	requests, err := pd.proxyUcase.GetAllRequests()
	if err != nil {
		logrus.Error(err)
	}

	var result string
	for _, request := range requests {
		result += "\nId: " + strconv.FormatInt(request.Id, 16) + "\nMethod: " + request.Method +
			"\nScheme: " + request.Scheme + "\nPath: " + request.Path + "\nHost: " + request.Host +
			"\nBody: " + request.Body + "\n"
	}
	_, err = io.Copy(writer, strings.NewReader(result))
	if err != nil {
		logrus.Info(err)
	}
}

func (pd *ProxyDelivery) GetRequestHandler(writer http.ResponseWriter, request *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(request)["id"], 10, 32)
	foundReq, err := pd.proxyUcase.GetRequest(id)
	if err != nil {
		logrus.Error(err)
	}

	result := "Id: " + strconv.FormatInt(foundReq.Id, 16) + "\nMethod: " + foundReq.Method +
		"\nScheme: " + foundReq.Scheme + "\nPath: " + foundReq.Path + "\nHost: " + foundReq.Host +
		"\nBody: " + foundReq.Body + "\n"

	_, err = io.Copy(writer, strings.NewReader(result))
	if err != nil {
		logrus.Info(err)
	}
}

func (pd *ProxyDelivery) RepeatRequestHandler(writer http.ResponseWriter, request *http.Request) {
	pd.needSave = false
	id, err := strconv.ParseInt(mux.Vars(request)["id"], 10, 32)
	foundReq, err := pd.proxyUcase.GetRequest(id)
	if err != nil {
		logrus.Error(err)
	}

	pd.HandleRequest(writer, &http.Request{
		Method: foundReq.Method,
		URL: &url.URL{
			Scheme: foundReq.Scheme,
			Host:   foundReq.Host,
			Path:   foundReq.Path,
		},
		Header: foundReq.Headers,
		Body:   ioutil.NopCloser(strings.NewReader(foundReq.Body)),
		Host:   request.Host,
	})
}

func (pd *ProxyDelivery) ScanRequestHandler(writer http.ResponseWriter, request *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(request)["id"], 10, 32)
	foundReq, err := pd.proxyUcase.GetRequest(id)
	if err != nil {
		logrus.Error(err)
	}

	err = pd.ScanRequest(writer, &http.Request{
		Method: foundReq.Method,
		URL: &url.URL{
			Scheme: foundReq.Scheme,
			Host:   foundReq.Host,
			Path:   foundReq.Path,
		},
		Header: foundReq.Headers,
		Body:   ioutil.NopCloser(strings.NewReader(foundReq.Body)),
		Host:   request.Host,
	})

	if err != nil {
		logrus.Error(err)
	}
}

func (pd *ProxyDelivery) ScanRequest(writer http.ResponseWriter, request *http.Request) error {
	f, err := os.Open("paths.txt")

	if err != nil {
		logrus.Error(err)
		return nil
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanWords)

	path := request.URL.Path

	for scanner.Scan() {
		if path[len(path)-1] == '/' {
			request.URL.Path = path + scanner.Text()
		} else {
			request.URL.Path = path + "/" + scanner.Text()
		}

		fmt.Println(request.URL.Path)

		response, err := pd.proxyUcase.DoHttpRequest(request)
		if err != nil {
			logrus.Info(err)
		}

		if response.StatusCode != 404 {
			_, err = io.Copy(writer, strings.NewReader(request.URL.Path+"\t"+response.Status+"\n"))
			if err != nil {
				logrus.Error(err)
			}
		}

	}

	return nil
}
