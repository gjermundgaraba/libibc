package skipapi

type Client struct {
	baseUrl string
}

func NewClient(baseUrl string) *Client {
	return &Client{
		baseUrl: baseUrl,
	}
}
