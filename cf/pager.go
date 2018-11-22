package cf

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry-community/go-cfclient"
)

type ResponseParser func(*cfclient.ServicePlanVisibilitiesResponse) (interface{}, error)

type PageResponse struct {
	*cfclient.Client
	currentPage int
	nextURL     string
	result      interface{}
	parse       ResponseParser
}

func NewPager(c *cfclient.Client, requestURL string, parse ResponseParser) (*PageResponse) {
	return &PageResponse{
		Client:      c,
		parse: parse,
		nextURL:     requestURL,
		currentPage: 0,
	}
}

func (p *PageResponse) Next(ctx context.Context) error {
	if p.nextURL == "" {
		return nil
	}

	result, err := doRequest(p.Client, p.nextURL)
	if err != nil {
		return err
	}

	resources, err := p.parse(result)
	if err != nil {
		return err
	}
	p.result = resources

	p.currentPage++
	p.nextURL = result.NextUrl

	return nil
}

func doRequest(c *cfclient.Client, requestURL string) (*cfclient.ServicePlanVisibilitiesResponse, error) {
	req := c.NewRequest("GET", requestURL)
	resp, err := c.DoRequest(req)
	if err != nil {
		return nil, err
	}

	var result cfclient.ServicePlanVisibilitiesResponse
	if err = unmarshalResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func unmarshalResponse(resp *http.Response, result *cfclient.ServicePlanVisibilitiesResponse) error {
	resBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(resBody, result)
	if err != nil {
		return err
	}
	return nil
}

func (p *PageResponse) GetResult() interface{} {
	return p.result
}

func (p *PageResponse) HasNext() bool {
	return p.nextURL != ""
}
