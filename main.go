package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

const (
	ENABLE         = "on"
	MqttSenderHost = "MQTT_SENDER_HOST"
	HomeSonoff     = "home/sonoff/state"
)

type devices []device

type device struct {
	DeviceId string `json:"id" binding:"required"`
	Name     string `json:"name"`
	Host     string `json:"host" binding:"required"`
	Enable   bool   `json:"enable"`
}

type errorMsg struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Switcher struct {
	DeviceId string `json:"deviceid"`
	Data     struct {
		Switch string `json:"switch"`
	} `json:"data"`
}

type Message struct {
	Topic    string `json:"topic"`
	Qos      int    `json:"qos"`
	Retained bool   `json:"retained"`
	Payload  string `json:"payload"`
}

func init() {
	_, isPresent := os.LookupEnv("DEVICES_FILE")
	if !isPresent {
		_ = os.Setenv("DEVICES_FILE", "config/devices.json")
	}
	fmt.Println("DEVICES_FILE:", os.Getenv("DEVICES_FILE"))
}

func main() {
	r := gin.Default()
	r.Use(sendMqttMessage())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": http.StatusText(http.StatusOK),
		})
	})

	r.GET("/devices", func(c *gin.Context) {
		devices, err := getDevices()
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorMsg{
				Status:  http.StatusText(http.StatusInternalServerError),
				Message: err.Error(),
			})
		} else {
			c.JSON(http.StatusOK, devices)
		}
	})

	r.POST("/device/:id/*proxyPath", proxy)

	_ = r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func getDevices() (d devices, err error) {
	jsonFile, err := os.Open(os.Getenv("DEVICES_FILE"))
	if err != nil {
		fmt.Println("error opening file ["+os.Getenv("DEVICES_FILE")+"]:", err)
		return
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println("error reading JSON:", err)
		return
	}

	err = json.Unmarshal(byteValue, &d)
	if err != nil {
		fmt.Println("error mapping from JSON to object:", err)
		return
	}
	return
}

func proxy(c *gin.Context) {
	id := c.Param("id")
	device, err := getHostById(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorMsg{
			Status:  http.StatusText(http.StatusInternalServerError),
			Message: err.Error(),
		})
		return
	}

	if device.Host == "" {
		c.JSON(http.StatusBadRequest, errorMsg{
			Status:  http.StatusText(http.StatusBadRequest),
			Message: "invalid device id: " + id,
		})
		return
	}

	if device.Enable == false {
		c.JSON(http.StatusConflict, errorMsg{
			Status:  http.StatusText(http.StatusConflict),
			Message: "The device is turned off: " + id,
		})
		return
	}

	remote, err := url.Parse("http://" + device.Host)
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusBadRequest, errorMsg{
			Status:  http.StatusText(http.StatusBadRequest),
			Message: err.Error(),
		})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.URL.Path = c.Param("proxyPath")
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func sendMqttMessage() gin.HandlerFunc {
	return func(c *gin.Context) {
		var bodyBytes []byte
		var jsonBody Switcher

		if c.Request.Body != nil {
			bodyBytes, _ = ioutil.ReadAll(c.Request.Body)
			c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
			err := json.Unmarshal(bodyBytes, &jsonBody)
			if err == nil && strings.HasSuffix(c.Request.RequestURI, "zeroconf/switch") && jsonBody.Data.Switch != "" {
				msg := Message{
					Topic:    HomeSonoff,
					Qos:      2,
					Retained: false,
					Payload:  fmt.Sprintf("%s,id=%s value=%v", "sonoff", c.Request.RequestURI[8:18], getState(jsonBody.Data.Switch)),
				}
				err = sendMessage(msg)
				if err != nil {
					fmt.Println("error sending mqtt:", err)
				}
			}
		}
	}
}

func getState(state string) (v bool) {
	v = false
	if state == ENABLE {
		v = true
	}
	return
}

func sendMessage(message Message) error {
	uri := fmt.Sprintf("http://%s/publish", getMqttSenderHost())
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(message)
	if err != nil {
		return fmt.Errorf("%q: %v", uri, err)
	}
	resp, err := http.Post(uri, "application/json; charset=utf-8", body)
	if err != nil {
		return fmt.Errorf("cannot fetch URL %q: %v", uri, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected http POST status: %s", resp.Status)
	}
	return nil
}

func getMqttSenderHost() string {
	return os.Getenv(MqttSenderHost)
}

func getHostById(id string) (device device, err error) {
	devices, err := getDevices()
	if err == nil {
		for _, d := range devices {
			if d.DeviceId == id {
				device = d
			}
		}
	}
	return
}
