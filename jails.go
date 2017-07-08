package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type Jail struct {
	AllowRawSockets  string
	AllowMount       string
	AllowSetHostname string
	AllowSysVIPC     string
	Clean            string
	ConsoleLog       string
	Hostname         string
	IPV4Addr         string
	JailUser         string
	JailName         string
	Path             string
	SystemUser       string
	Start            string
	Stop             string
	Template         string
	UseDefaults      string
}

type CreateJailResponse struct {
	Message string
	Error   error
	JUID    string
}

type ListJailsResponse struct {
	Message string
	Error   error
	Jails   []Jail
}

type GetJailResponse struct {
	Message string
	Error   error
	Jails   Jail
}

type JailState struct {
	JailName string
	State    string // Running|Stopped
}

const ( // = example line:
	AllowRawSocketsLine  = `allow.raw_sockets = `  // allow.raw_sockets = 0;
	AllowMountLine       = `allow.mount;`          // allow.mount;
	AllowSetHostnameLine = `allow.set_hostname = ` // allow.set_hostname = 0;
	AllowSysVIPCLine     = `allow.sysvipc = `      // allow.sysvipc = 0;
	CleanLine            = `exec.clean;`           // exec.clean;
	ConsoleLogLine       = `exec.consolelog = `    // exec.consolelog = "/var/log/jail_${name}_console.log";
	HostnameLine         = `host.hostname = `      // host.hostname = "pie.local";
	IPV4AddrLine         = `ip4.addr = `           // ip4.addr = 10.0.2.12;
	JailUserLine         = `exec.jail_user = `     // exec.jail_user = "root";
	PathLine             = `path = `               // path = "/usr/jail/${name}";
	SystemUserLine       = `exec.system_user = `   // exec.system_user = "root";
	StartLine            = `exec.start += `        // exec.start += "/bin/sh /etc/rc";
	StopLine             = `exec.stop = `          // exec.stop = "/bin/sh /etc/rc.shutdown";
)

var bucketName = []byte("jails")

func validForm(bucketName []byte, reqForm Jail) error {
	err := JestDB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			encoded := bytes.NewReader(v)
			form := Jail{}
			err := json.NewDecoder(encoded).Decode(&form)
			if err != nil {
				log.Warn("Couldn't decode a key:", err)
			}

			switch {
			case form.Hostname == reqForm.Hostname:
				return fmt.Errorf("Hostname already in use: " + reqForm.Hostname + ".")
			case form.JailName == reqForm.JailName:
				return fmt.Errorf("Jail name already in use: " + reqForm.JailName + ".")
			case form.IPV4Addr == reqForm.IPV4Addr:
				return fmt.Errorf("IP address already in use: " + reqForm.IPV4Addr + ".")
			}
		}

		return nil
	})

	return err
}

func CreateJailsEndpoint(w http.ResponseWriter, r *http.Request) {
	jUID := uuid.NewV4()

	var form Jail
	log.Info("Received a create jail request from " + r.RemoteAddr)

	HostNotInitialised(w, r)

	log.Debug("Decoding the JSON request.")
	err := json.NewDecoder(r.Body).Decode(&form)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"Failed to decode the JSON request", err, jUID.String()}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"request": form, "error": err, "jUID": jUID.String()}).Warn(res.Message)
		return
	}
	log.WithFields(log.Fields{"request": form, "jUID": jUID.String()}).Debug("Decoded JSON request.")

	bucketName := []byte("jails")

	switch {
	case form.JailName == "":
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"No jail name supplied.", fmt.Errorf("You must supply a jail name to be used."), jUID.String()}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error, "jUID": jUID.String()}).Warn(res.Message)
		return
	case form.Template == "":
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"No template supplied.", fmt.Errorf("You must include a template with the request, the template is the name of the base jail you wish to clone."), jUID.String()}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error, "jUID": jUID.String()}).Warn(res.Message)
		return
	case form.Hostname == "":
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"No hostname supplied.", fmt.Errorf("You must include a hostname with the request."), jUID.String()}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error, "jUID": jUID.String()}).Warn(res.Message)
		return
	case form.IPV4Addr == "":
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"No IP address supplied.", fmt.Errorf("You must include a IP with the request."), jUID.String()}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error, "jUID": jUID.String()}).Warn(res.Message)
		return
	}

	defaults := Jail{
		`0`,
		`0`,
		`0`,
		`0`,
		`0`,
		`/var/log/jail_${name}_console.log`,
		form.Hostname,
		form.IPV4Addr,
		"root",
		form.JailName,
		"/usr/jail",
		"root",
		`/bin/sh /etc/rc`,
		`/bin/sh /etc/rc.shutdown`,
		form.Template,
		form.UseDefaults,
	}

	err = validForm(bucketName, form)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"Invalid form.", err, jUID.String()}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error, "jUID": jUID.String()}).Warn(res.Message)
		return
	}

	if form.UseDefaults == "true" {
		encoded, err := json.Marshal(defaults)
		if err != nil {
			log.WithFields(log.Fields{"error": err, "jUID": jUID.String()}).Warn("Failed to encode the struct to JSON before writing to the JestDB.")
		}
		JestDB.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucketName)
			err := b.Put(jUID.Bytes(), encoded)
			return err
		})
	} else {
		encoded, err := json.Marshal(form)
		if err != nil {
			log.WithFields(log.Fields{"error": err, "jUID": jUID.String()}).Warn("Failed to encode the struct to JSON before writing to the JestDB.")
		}
		JestDB.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucketName)
			err := b.Put(jUID.Bytes(), encoded)
			return err
		})
	}

	JestDB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		v := b.Get(jUID.Bytes())

		json.NewDecoder(bytes.NewReader(v)).Decode(&form)
		return nil
	})

	res := CreateJailResponse{"Jail created successfully", nil, jUID.String()}
	log.WithFields(log.Fields{"error": res.Error, "jUID": res.JUID}).Info(res.Message)
	json.NewEncoder(w).Encode(res)
	return
}

func listAllJails() []Jail {
	var jails = []Jail{}

	JestDB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			encoded := bytes.NewReader(v)
			form := Jail{}
			err := json.NewDecoder(encoded).Decode(&form)
			if err != nil {
				log.Warn("Couldn't decode a key:", err)
			}

			jails = append(jails, form)

		}
		return nil
	})

	return jails
}

func ListJailsEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Info("Received a get jails request from " + r.RemoteAddr)
	HostNotInitialised(w, r)

	jails := listAllJails()

	if len(jails) < 1 {
		w.WriteHeader(http.StatusNotFound)
		res := ListJailsResponse{"No jails found.", fmt.Errorf("There are no jails enabled on this device."), jails}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	w.WriteHeader(http.StatusOK)
	res := ListJailsResponse{"Jails found.", nil, jails}
	log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
	json.NewEncoder(w).Encode(res)
	return
}

func GetJailEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Info("Received a get jail request from " + r.RemoteAddr)
	vars := mux.Vars(r)
	HostNotInitialised(w, r)

	jails := listAllJails()

	if len(jails) < 1 {
		w.WriteHeader(http.StatusNotFound)
		res := GetJailResponse{"No jails found.", fmt.Errorf("There are no jails enabled on this device."), Jail{}}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	for j := range jails {
		if jails[j].JailName == vars["name"] {
			w.WriteHeader(http.StatusOK)
			res := GetJailResponse{"Jail found.", nil, jails[j]}
			log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
			json.NewEncoder(w).Encode(res)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	res := GetJailResponse{"Jail not found.", fmt.Errorf("There is no jail on this host with the name " + vars["name"]), Jail{}}
	log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
	json.NewEncoder(w).Encode(res)
	return
}

func startJail(jail Jail, state string) {

}

func ChangeJailStateEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Info("Received a change jail state request from " + r.RemoteAddr)
	HostNotInitialised(w, r)
}

func DeleteJailEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Info("Received a change jail state request from " + r.RemoteAddr)
	HostNotInitialised(w, r)
}
