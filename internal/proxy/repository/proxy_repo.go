package repository

import (
	"encoding/json"

	"github.com/Snikimonkd/mitm_proxy/internal/models"
	"github.com/Snikimonkd/mitm_proxy/internal/proxy"
	"github.com/jmoiron/sqlx"
)

type ProxyRepos struct {
	conn *sqlx.DB
}

func NewProxyRepos(conn *sqlx.DB) proxy.ProxyRepository {
	return &ProxyRepos{
		conn: conn,
	}
}

func (pr *ProxyRepos) InsertInto(request *models.Request) error {
	bs, err := json.Marshal(request.Headers)
	if err != nil {
		return err
	}

	if err := pr.conn.QueryRow(
		`INSERT INTO requests(method, scheme, host, path, headers, body) 
				VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		request.Method, request.Scheme, request.Host, request.Path, string(bs), request.Body).Scan(&request.Id); err != nil {
		return err
	}

	return nil
}

func (pr *ProxyRepos) GetRequest(id int64) (*models.Request, error) {
	rows, err := pr.conn.Queryx(`SELECT * from requests where id = $1`, id)
	if err != nil {
		return nil, err
	}

	requests, err := pr.scanRequests(rows)
	if err != nil {
		return nil, err
	}

	if len(requests) == 0 {
		return nil, nil
	}

	return requests[0], nil
}
func (pr *ProxyRepos) GetAllRequests() ([]*models.Request, error) {
	rows, err := pr.conn.Queryx(`SELECT * from requests`)
	if err != nil {
		return nil, err
	}

	requests, err := pr.scanRequests(rows)
	if err != nil {
		return nil, err
	}

	return requests, nil
}

func (pr *ProxyRepos) scanRequests(rows *sqlx.Rows) ([]*models.Request, error) {
	var requests []*models.Request
	for rows.Next() {
		reqMap := make(map[string]interface{})
		err := rows.MapScan(reqMap)
		if err != nil {
			return nil, err
		}

		headersRaw := reqMap["headers"].([]byte)

		var headers map[string][]string
		err = json.Unmarshal(headersRaw, &headers)
		if err != nil {
			return nil, err
		}

		requests = append(requests, &models.Request{
			Id:      reqMap["id"].(int64),
			Method:  reqMap["method"].(string),
			Scheme:  reqMap["scheme"].(string),
			Host:    reqMap["host"].(string),
			Path:    reqMap["path"].(string),
			Headers: headers,
			Body:    reqMap["body"].(string),
		})
	}

	return requests, nil
}
