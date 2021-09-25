package main

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

const config = `
data "http" "http_test" {
  url = "%s"
  method = "%s"
  body = "%s"
}
output "response_body" {
  value = data.http.http_test.response_body
}
output "response_headers" {
  value = data.http.http_test.response_headers
}
`

func assert(expected, actual interface{}) error {
	if !reflect.DeepEqual(expected, actual) {
		return fmt.Errorf(
			"expected %v (type %v), actual %v (type %v)",
			expected,
			reflect.TypeOf(expected),
			actual,
			reflect.TypeOf(actual),
		)
	}
	return nil
}

type testCase struct {
	requestBody     string
	requestMethod   string
	responseHeaders map[string]string
	responseBody    string
	responseStatus  int
	err             string
}

func run(t *testing.T, c testCase) {
	if c.requestMethod == "" {
		c.requestMethod = http.MethodGet
	}
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range c.responseHeaders {
				for _, v := range strings.Split(v, ", ") {
					w.Header().Add(k, v)
				}
			}
			if c.err == "" {
				if r.Method != http.MethodGet {
					payload, err := ioutil.ReadAll(r.Body)
					if err != nil {
						panic(err)
					}
					w.Write([]byte(fmt.Sprintf("%s %s", r.Method, payload)))
				} else {
					w.Write([]byte(c.responseBody))
				}
			} else {
				w.WriteHeader(c.responseStatus)
			}
		}),
	)
	defer server.Close()
	step := resource.TestStep{}
	step.Config = fmt.Sprintf(config, server.URL, c.requestMethod, c.requestBody)
	if c.err != "" {
		step.ExpectError = regexp.MustCompile(c.err)
	} else {
		step.Check = func(s *terraform.State) error {
			if _, ok := s.RootModule().Resources["data.http.http_test"]; !ok {
				return fmt.Errorf("missing data resource")
			}
			outputs := s.RootModule().Outputs
			response_headers := make(map[string]string)
			for k, v := range outputs["response_headers"].Value.(map[string]interface{}) {
				response_headers[k] = v.(string)
			}
			delete(response_headers, "Content-Length")
			delete(response_headers, "Content-Type")
			delete(response_headers, "Date")
			response_body := outputs["response_body"].Value
			if err := assert(c.responseBody, response_body); err != nil {
				return err
			}
			return assert(c.responseHeaders, response_headers)
		}
	}
	resource.UnitTest(t, resource.TestCase{
		Providers: map[string]*schema.Provider{
			"http": Provider(),
		},
		Steps: []resource.TestStep{step},
	})
}

func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestDataSource_Simple(t *testing.T) {
	run(t, testCase{
		responseHeaders: map[string]string{
			"X-Test": "test",
		},
		responseBody: "test",
	})
}

func TestDataSource_Headers(t *testing.T) {
	run(t, testCase{
		responseHeaders: map[string]string{
			"X-Test":       "test",
			"X-Test-Multi": "test-1, test-2",
		},
		responseBody: "test",
	})
}

func TestDataSource_NotFound(t *testing.T) {
	run(t, testCase{
		responseStatus: http.StatusNotFound,
		err:            "HTTP status code does not match: expected 200 actual 404",
	})
}

func TestDataSource_Post(t *testing.T) {
	run(t, testCase{
		requestMethod: http.MethodPost,
		requestBody:   "payload",
		responseHeaders: map[string]string{
			"X-Test": "test",
		},
		responseBody: "POST payload",
	})
}
