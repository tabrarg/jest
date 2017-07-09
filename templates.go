package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type Template struct {
	Name      string
	Disabled  bool
	Path      string
	Version   string
	ZFSParams ZFSParams
}

type TemplatesResponse struct {
	Message   string
	Error     error
	Templates []Template
}

type TemplateResponse struct {
	Message  string
	Error    error
	Template Template
}

func CreateTemplate(template Template) {

}

func listAllTemplates() []Template {
	var templates = []Template{}

	JestDB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("templates"))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			encoded := bytes.NewReader(v)
			form := Template{}
			err := json.NewDecoder(encoded).Decode(&form)
			if err != nil {
				log.Warn("Couldn't decode a key:", err)
			}

			templates = append(templates, form)

		}
		return nil
	})

	return templates
}

func ListTemplatesEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Info("Received a get template request from " + r.RemoteAddr)
	HostNotInitialised(w, r)

	templates := listAllTemplates()

	if len(templates) < 1 {
		w.WriteHeader(http.StatusNotFound)
		res := TemplatesResponse{"No templates found.", fmt.Errorf("There are no templates enabled on this host."), templates}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	w.WriteHeader(http.StatusOK)
	res := TemplatesResponse{"Templates found.", nil, templates}
	log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
	json.NewEncoder(w).Encode(res)
	return
}

func getTemplate(templateName string, templates []Template) (Template, error) {
	for t := range templates {
		if templates[t].Name == templateName {
			return templates[t], nil
		}
	}
	return Template{}, fmt.Errorf("There is not template on this host with the name " + templateName + ".")
}

func GetTemplateEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Info("Received a get template request from " + r.RemoteAddr)
	vars := mux.Vars(r)
	HostNotInitialised(w, r)

	templates := listAllTemplates()

	if len(templates) < 1 {
		w.WriteHeader(http.StatusNotFound)
		res := TemplateResponse{"No template found.", fmt.Errorf("There are no template enabled on this host."), Template{}}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	template, err := getTemplate(vars["name"], templates)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		res := TemplateResponse{"Template not found.", fmt.Errorf("There is no template on this host with the name " + vars["name"]), Template{}}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	w.WriteHeader(http.StatusOK)
	res := TemplateResponse{"Template found.", nil, template}
	log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
	json.NewEncoder(w).Encode(res)
	return
}
