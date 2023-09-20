package storage_go

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
)

const (
	defaultLimit            = 100
	defaultOffset           = 0
	defaultFileCacheControl = "3600"
	defaultFileContentType  = "text/plain;charset=UTF-8"
	defaultFileUpsert       = false
	defaultSortColumn       = "name"
	defaultSortOrder        = "asc"
)

func (c *Client) UploadOrUpdateFile(bucketId string, relativePath string, data io.Reader, update bool) (FileUploadResponse, error) {
	c.clientTransport.header.Set("cache-control", defaultFileCacheControl)
	if c.clientTransport.header.Get("content-type") == "" {
		c.clientTransport.header.Set("content-type", defaultFileContentType)
	}
	c.clientTransport.header.Set("x-upsert", strconv.FormatBool(defaultFileUpsert))
	body := bufio.NewReader(data)
	_path := removeEmptyFolderName(bucketId + "/" + relativePath)

	var (
		res     *http.Response
		err     error
		request *http.Request
		method  = http.MethodPost
	)

	if update {
		method = http.MethodPut
	}

	request, err = http.NewRequest(method, c.clientTransport.baseUrl.String()+"/object/"+_path, body)
	if err != nil {
		return FileUploadResponse{}, err
	}
	res, err = c.session.Do(request)
	if err != nil {
		return FileUploadResponse{}, err
	}

	body_, err := io.ReadAll(res.Body)
	if err != nil {
		return FileUploadResponse{}, err
	}
	var response FileUploadResponse
	err = json.Unmarshal(body_, &response)
	if err != nil {
		return FileUploadResponse{}, err
	}

	return response, nil
}

func (c *Client) UpdateFile(bucketId string, relativePath string, data io.Reader) (FileUploadResponse, error) {
	return c.UploadOrUpdateFile(bucketId, relativePath, data, true)
}

func (c *Client) UploadFile(bucketId string, relativePath string, data io.Reader) (FileUploadResponse, error) {
	return c.UploadOrUpdateFile(bucketId, relativePath, data, false)
}

func (c *Client) MoveFile(bucketId string, sourceKey string, destinationKey string) (FileUploadResponse, error) {
	jsonBody, _ := json.Marshal(map[string]interface{}{
		"bucketId":       bucketId,
		"sourceKey":      sourceKey,
		"destinationKey": destinationKey,
	})

	request, err := http.NewRequest(
		http.MethodPost,
		c.clientTransport.baseUrl.String()+"/object/move",
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return FileUploadResponse{}, err
	}

	res, err := c.session.Do(request)
	if err != nil {
		return FileUploadResponse{}, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return FileUploadResponse{}, err
	}
	var response FileUploadResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return FileUploadResponse{}, err
	}

	return response, nil
}

func (c *Client) CreateSignedUrl(bucketId string, filePath string, expiresIn int) (SignedUrlResponse, error) {
	jsonBody, _ := json.Marshal(map[string]interface{}{
		"expiresIn": expiresIn,
	})

	request, err := http.NewRequest(
		http.MethodPost,
		c.clientTransport.baseUrl.String()+"/object/sign/"+bucketId+"/"+filePath,
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return SignedUrlResponse{}, err
	}

	res, err := c.session.Do(request)
	if err != nil {
		return SignedUrlResponse{}, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return SignedUrlResponse{}, err
	}
	var response SignedUrlResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return SignedUrlResponse{}, err
	}
	response.SignedURL = c.clientTransport.baseUrl.String() + response.SignedURL

	return response, nil
}

func (c *Client) CreateSignedUploadUrl(bucketId string, filePath string) (SignedUploadUrlResponse, error) {
	emptyBody, _ := json.Marshal(struct{}{})
	request, err := http.NewRequest(
		http.MethodPost,
		c.clientTransport.baseUrl.String()+"/object/upload/sign/"+bucketId+"/"+filePath, bytes.NewBuffer(emptyBody))
	if err != nil {
		return SignedUploadUrlResponse{}, err
	}
	res, err := c.session.Do(request)
	if err != nil {
		return SignedUploadUrlResponse{}, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return SignedUploadUrlResponse{}, err
	}
	var response SignedUploadUrlResponse
	err = json.Unmarshal(body, &response)

	return response, err
}

func (c *Client) UploadToSignedUrl(filePath string, fileBody io.Reader) (*UploadToSignedUrlResponse, error) {
	c.clientTransport.header.Set("cache-control", defaultFileCacheControl)
	c.clientTransport.header.Set("content-type", defaultFileContentType)
	c.clientTransport.header.Set("x-upsert", strconv.FormatBool(defaultFileUpsert))

	bodyRequest := bufio.NewReader(fileBody)
	path := removeEmptyFolderName(filePath)

	request, err := http.NewRequest(http.MethodPut, c.clientTransport.baseUrl.String()+path, bodyRequest)
	if err != nil {
		return nil, err
	}
	resp, err := c.session.Do(request)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response UploadToSignedUrlResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, err
}

func (c *Client) GetPublicUrl(bucketId string, filePath string, urlOptions ...UrlOptions) SignedUrlResponse {
	var response SignedUrlResponse
	renderPath := "object"
	var options UrlOptions
	if len(urlOptions) > 0 {
		options = urlOptions[0]
		if options.Transform != nil {
			renderPath = "render/image"
		}
	}
	urlStr := c.clientTransport.baseUrl.String() + "/" + renderPath + "/public/" + bucketId + "/" + filePath
	response.SignedURL = buildUrlWithOption(urlStr, options)

	return response
}

func (c *Client) RemoveFile(bucketId string, paths []string) (FileUploadResponse, error) {
	jsonBody, _ := json.Marshal(map[string]interface{}{
		"prefixes": paths,
	})

	request, err := http.NewRequest(
		http.MethodDelete,
		c.clientTransport.baseUrl.String()+"/object/"+bucketId,
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return FileUploadResponse{}, err
	}

	res, err := c.session.Do(request)
	if err != nil {
		return FileUploadResponse{}, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return FileUploadResponse{}, err
	}
	var response FileUploadResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return FileUploadResponse{}, err
	}
	response.Data = body

	return response, nil
}

func (c *Client) ListFiles(bucketId string, queryPath string, options FileSearchOptions) ([]FileObject, error) {
	if options.Offset == 0 {
		options.Offset = defaultOffset
	}

	if options.Limit == 0 {
		options.Limit = defaultLimit
	}

	if options.SortByOptions.Order == "" {
		options.SortByOptions.Order = defaultSortOrder
	}

	if options.SortByOptions.Column == "" {
		options.SortByOptions.Column = defaultSortColumn
	}

	body_ := ListFileRequestBody{
		Limit:  options.Limit,
		Offset: options.Offset,
		SortByOptions: SortBy{
			Column: options.SortByOptions.Column,
			Order:  options.SortByOptions.Order,
		},
		Prefix: queryPath,
	}
	jsonBody, _ := json.Marshal(body_)

	request, err := http.NewRequest(
		http.MethodPost,
		c.clientTransport.baseUrl.String()+"/object/list/"+bucketId,
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	res, err := c.session.Do(request)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var response []FileObject

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (c *Client) DownloadFile(bucketId string, filePath string, urlOptions ...UrlOptions) ([]byte, error) {
	var options UrlOptions
	renderPath := "object"
	if len(urlOptions) > 0 {
		options = urlOptions[0]
		if options.Transform != nil {
			renderPath = "render/image/authenticated"
		}
	}
	urlStr := c.clientTransport.baseUrl.String() + "/" + renderPath + "/" + bucketId + "/" + filePath
	request, err := http.NewRequest(
		http.MethodGet,
		buildUrlWithOption(urlStr, options),
		nil)
	if err != nil {
		return nil, err
	}
	res, err := c.session.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	return body, err
}

// buildUrlWithOption will base on current url and option to build a new url
func buildUrlWithOption(urlStr string, options UrlOptions) string {
	signedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	signedURLQuery := signedURL.Query()

	if options.Transform != nil {
		if options.Transform.Height > 0 {
			signedURLQuery.Add("height", strconv.Itoa(options.Transform.Height))
		}
		if options.Transform.Width > 0 {
			signedURLQuery.Add("width", strconv.Itoa(options.Transform.Width))
		}
		// Default: origin
		if options.Transform.Format != "" {
			signedURLQuery.Add("format", options.Transform.Format)
		}
		// Default: 80
		if options.Transform.Quality > 0 {
			signedURLQuery.Add("quality", strconv.Itoa(options.Transform.Quality))
		}
		if options.Transform.Resize != "" && (options.Transform.Resize == "conver" || options.Transform.Resize == "contain" || options.Transform.Resize == "fill") {
			signedURLQuery.Add("resize", options.Transform.Resize)
		}
	}
	// Default on server is false
	if options.Download == true {
		signedURLQuery.Add("download", strconv.FormatBool(options.Download))
	}

	signedURL.RawQuery = signedURLQuery.Encode()
	return signedURL.String()
}

// removeEmptyFolderName replaces occurances of double slashes (//)  with a single slash /
// returns a path string with all double slashes replaced with single slash /
func removeEmptyFolderName(filePath string) string {
	return regexp.MustCompile(`\/\/`).ReplaceAllString(filePath, "/")
}

type SortBy struct {
	Column string `json:"column"`
	Order  string `json:"order"`
}

type FileUploadResponse struct {
	Key     string `json:"Key"`
	Message string `json:"message"`
	Data    []byte
	Code    string `json:"statusCode"`
	Error   string `json:"error"`
}

type SignedUrlResponse struct {
	SignedURL string `json:"signedURL"`
}

type FileSearchOptions struct {
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
	SortByOptions SortBy `json:"sortBy"`
}

type FileObject struct {
	Name           string      `json:"name"`
	BucketId       string      `json:"bucket_id"`
	Owner          string      `json:"owner"`
	Id             string      `json:"id"`
	UpdatedAt      string      `json:"updated_at"`
	CreatedAt      string      `json:"created_at"`
	LastAccessedAt string      `json:"last_accessed_at"`
	Metadata       interface{} `json:"metadata"`
	Buckets        Bucket
}

type ListFileRequestBody struct {
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
	SortByOptions SortBy `json:"sortBy"`
	Prefix        string `json:"prefix"`
}

type TransformOptions struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Resize  string `json:"resize"`
	Format  string `json:"format"`
	Quality int    `json:"quality"`
}

type UrlOptions struct {
	Transform *TransformOptions `json:"transform"`
	Download  bool              `json:"download"`
}

type SignedUploadUrlResponse struct {
	Url string `json:"url"`
}

type UploadToSignedUrlResponse struct {
	Key string `json:"key"`
}
