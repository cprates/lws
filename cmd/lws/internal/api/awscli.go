package api

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

type AwsCli struct {
	actions map[string]actionFunc
}

type actionFunc func(params url.Values, method string) error

func NewAwsCli() AwsCli {
	return AwsCli{
		actions: map[string]actionFunc{},
	}
}

func (a AwsCli) regAction(id string, f actionFunc) AwsCli {

	a.actions[id] = f
	return a
}

// TODO: this must go to a aws related folder. Server has nothig to do with this checks aws related. We might have
// other endpoints with other functionality like admin stuff:
// pkg/cli/dispatcher.go
// pkg/cli/errcom.go
// pkg/cli/sqs.go
func (a AwsCli) Dispatcher() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// TODO: debug
		fmt.Println("URI:", r.RequestURI)
		fmt.Println("URL:", r.URL)
		fmt.Println("Method:", r.Method)

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("Failed to read body,", err)
			return
		}

		// TODO: debug
		log.Println(string(body))

		params, err := url.ParseQuery(string(body))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("Failed to read query params,", err)
			return
		}

		id := params.Get("Action")
		if id == "" {
			//w.WriteHeader(http.StatusBadRequest)
			// TODO: try to call the real aws endpoint to check the response
			//w.Write([]byte("'Action' not specified"))
			log.Println("Query param Action not specified")
			onError(w, 400, "")
			return
		}

		actionF, ok := a.actions[id]
		if !ok {
			//w.WriteHeader(http.StatusBadRequest)
			//w.Write([]byte("unknown Action: " + id))
			log.Println("Unknown Action:", id)
			onError(w, http.StatusBadRequest, "unknown Action "+id)
			return
		}

		err = actionF(params, r.Method)
		if err != nil {
			log.Println("Failed serving req XXX,", err) // TODO: add req ID for all errors, and retur it in the message
			onError(w, http.StatusBadRequest, err.Error())
		}
	}
}

type ErrorResult struct {
	Code      string `xml:"Code,omitempty"`
	Message   string `xml:"Message,omitempty"`
	RequestId string `xml:"RequestId,omitempty"`
	Type      string `xml:"Type,omitempty"`
}

type ErrorResponse struct {
	Result ErrorResult `xml:"Error"`
}

// TODO
// Make a list of common errors to use directly
func onError(w http.ResponseWriter, status int, err string) {
	respStruct := ErrorResponse{ErrorResult{Type: "GeneralError", Code: "AWS.SimpleQueueService.GeneralError", Message: err, RequestId: "00000000-0000-0000-0000-000000000000"}}

	w.WriteHeader(status)
	enc := xml.NewEncoder(w)
	enc.Indent("  ", "    ")
	if err := enc.Encode(respStruct); err != nil {
		log.Printf("error: %v\n", err)
	}
}
