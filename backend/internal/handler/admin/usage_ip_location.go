package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// IPLocation looks up the recorded IP, without probing the proxy again.
func (h *UsageHandler) IPLocation(c *gin.Context) {
	ip, err := netip.ParseAddr(c.Query("ip"))
	if err != nil || !ip.Unmap().IsGlobalUnicast() || ip.Unmap().IsPrivate() {
		response.BadRequest(c, "A public IP address is required")
		return
	}
	client := &http.Client{
		Timeout:       8 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet,
		"https://ipwho.is/"+ip.Unmap().String(), nil)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "IP location lookup failed")
		return
	}
	res, err := client.Do(req)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "IP location service unavailable")
		return
	}
	defer res.Body.Close()
	var result struct {
		Success    bool   `json:"success"`
		Country    string `json:"country"`
		Region     string `json:"region"`
		City       string `json:"city"`
		Connection struct {
			ISP string `json:"isp"`
		} `json:"connection"`
	}
	if res.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(res.Body, 64*1024)).Decode(&result) != nil || !result.Success {
		response.Error(c, http.StatusBadGateway, "IP location lookup failed")
		return
	}
	response.Success(c, gin.H{
		"ip": ip.Unmap().String(), "country": result.Country,
		"region": result.Region, "city": result.City, "isp": result.Connection.ISP,
	})
}
