package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"io/ioutil"
	"net/http"
	"strings"
)

func read(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	url := data.Get("url").(string)
	headers := data.Get("headers").(map[string]interface{})
	method := data.Get("method").(string)
	body := data.Get("body").(string)
	status := data.Get("status").(int)

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return diag.Errorf("Error creating http request: %s", err)
	}
	for name, value := range headers {
		req.Header.Set(name, value.(string))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return diag.Errorf("Error performing http request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != status {
		return diag.Errorf("HTTP status code does not match: expected %d actual %d", status, resp.StatusCode)
	}

	hash := sha1.New()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return diag.FromErr(err)
	}
	hash.Write(bytes)
	responseHeaders := make(map[string]string)
	for k, v := range resp.Header {
		responseHeaders[k] = strings.Join(v, ", ")
		hash.Write([]byte(responseHeaders[k]))
	}
	id := fmt.Sprintf("%x", hash.Sum(nil))

	data.Set("response_headers", responseHeaders)
	data.Set("response_body", string(bytes))
	data.SetId(id)
	return nil
}

func Provider() *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{},
		DataSourcesMap: map[string]*schema.Resource{
			"http": {
				Description: "Perform the provided http request and produce the http response headers and body.",
				ReadContext: read,
				Schema: map[string]*schema.Schema{
					"url": {
						Type:        schema.TypeString,
						Required:    true,
						Description: "The http request url.",
					},
					"method": {
						Type:        schema.TypeString,
						Optional:    true,
						Default:     http.MethodGet,
						Description: "The http request method.",
					},
					"headers": {
						Type:        schema.TypeMap,
						Optional:    true,
						Description: "The http request headers.",
					},
					"body": {
						Type:        schema.TypeString,
						Optional:    true,
						Description: "The http request body.",
					},
					"status": {
						Type:        schema.TypeInt,
						Optional:    true,
						Default:     http.StatusOK,
						Description: "The expected http response status code. The actual status code has to match this value. Defaults to 200.",
					},
					"response_headers": {
						Type:        schema.TypeMap,
						Computed:    true,
						Description: "The http response headers.",
					},
					"response_body": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "The http response body.",
					},
				},
			},
		},
	}
}

func main() {
	plugin.Serve(&plugin.ServeOpts{ProviderFunc: Provider})
}
