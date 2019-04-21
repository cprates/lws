package api

import (
	"encoding/xml"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/lerr"
)

func onAccessDenied(w http.ResponseWriter, res, reqID string) {

	msg := fmt.Sprintf("Access to the resource %s is denied.", res)
	err := lerr.Result{
		Result: lerr.Details{
			Type:      "Sender",
			Code:      "AccessDenied",
			Message:   msg,
			RequestID: reqID,
		},
	}
	writeErr(w, http.StatusForbidden, err)
}

func onLwsErr(w http.ResponseWriter, r common.Result) {

	w.WriteHeader(r.Status)
	enc := xml.NewEncoder(w)
	if err := enc.Encode(r.Err); err != nil {
		log.Errorln("Unexpected error encoding error message,", err)
	}
}

func onMissingAction(w http.ResponseWriter, reqID string) {

	err := lerr.Result{
		Result: lerr.Details{
			Type:      "Sender",
			Code:      "MissingAction",
			Message:   "Missing Action",
			RequestID: reqID,
		},
	}
	writeErr(w, http.StatusBadRequest, err)
}

func onInvalidParameterValue(w http.ResponseWriter, srv, reqID string) {

	err := lerr.Result{
		Result: lerr.Details{
			Type:      "Sender",
			Code:      "InvalidParameterValue",
			Message:   "Invalid or unsupported service " + srv,
			RequestID: reqID,
		},
	}
	writeErr(w, http.StatusBadRequest, err)
}

func writeErr(w http.ResponseWriter, status int, err lerr.Result) {

	w.WriteHeader(status)
	enc := xml.NewEncoder(w)
	if err := enc.Encode(err); err != nil {
		log.Errorln("Unexpected error encoding error message,", err)
	}
}
