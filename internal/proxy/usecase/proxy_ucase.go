package usecase

import (
	"io"
	"io/ioutil"
	"net/http"

	"github.com/Snikimonkd/mitm_proxy/internal/models"
	"github.com/Snikimonkd/mitm_proxy/internal/proxy"
	"github.com/Snikimonkd/mitm_proxy/internal/utils"
	"github.com/sirupsen/logrus"
	hs "github.com/yletamitlu/proxy/internal/https"
)

type ProxyUcase struct {
	proxyRepo proxy.ProxyRepository
}

func NewProxyUcase(repos proxy.ProxyRepository) proxy.ProxyUsecase {
	return &ProxyUcase{
		proxyRepo: repos,
	}
}

func (pu *ProxyUcase) HandleHttpRequest(writer http.ResponseWriter, interceptedHttpRequest *http.Request) (string, error) {
	proxyResponse, err := pu.DoHttpRequest(interceptedHttpRequest)
	if err != nil {
		logrus.Info(err)
	}
	for header, values := range proxyResponse.Header {
		for _, value := range values {
			writer.Header().Add(header, value)
		}
	}
	writer.WriteHeader(proxyResponse.StatusCode)
	_, err = io.Copy(writer, proxyResponse.Body)
	if err != nil {
		logrus.Info(err)
	}

	decodedResponse, err := utils.DecodeResponse(proxyResponse)
	if err != nil {
		return "", err
	}

	defer proxyResponse.Body.Close()
	return string(decodedResponse), nil
}

func (pu *ProxyUcase) HandleHttpsRequest(writer http.ResponseWriter, interceptedHttpRequest *http.Request, needSave bool) error {
	httpsService := hs.NewHttpsService(writer, interceptedHttpRequest, pu.proxyRepo)

	err := httpsService.ProxyHttpsRequest()
	if err != nil {
		return err
	}
	return nil
}

func (pu *ProxyUcase) DoHttpRequest(HttpRequest *http.Request) (*http.Response, error) {
	proxyRequest, err := http.NewRequest(HttpRequest.Method, HttpRequest.URL.String(), HttpRequest.Body)
	if err != nil {
		return nil, err
	}

	proxyRequest.Header = HttpRequest.Header

	proxyResponse, err := http.DefaultTransport.RoundTrip(proxyRequest)
	if err != nil {
		return nil, err
	}

	return proxyResponse, nil
}

func (pu *ProxyUcase) SaveReqToDB(request *http.Request, scheme string) error {
	bodyBytes, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return err
	}

	req := &models.Request{
		Method:  request.Method,
		Scheme:  scheme,
		Host:    request.Host,
		Path:    request.URL.Path,
		Headers: request.Header,
		Body:    string(bodyBytes),
	}

	err = pu.proxyRepo.InsertInto(req)
	if err != nil {
		return err
	}

	logrus.Info("RequestId: ", req.Id)

	return nil
}

func (pu *ProxyUcase) GetRequest(id int64) (*models.Request, error) {
	return pu.proxyRepo.GetRequest(id)
}

func (pu *ProxyUcase) GetAllRequests() ([]*models.Request, error) {
	return pu.proxyRepo.GetAllRequests()
}
