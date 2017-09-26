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
	"path/filepath"
	"os/exec"
)

type Jail struct {
	Name       string
	JailConfig JailConfig
	JailState JailState
}

type JailConfig struct {
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
	UseDefaults      bool
	//StartAtBoot   bool <- Need to think about how I will implement this
}

type JailState struct {
	Name    string
	Running bool
	JID     string
	// Processes Processes
}

type CreateJailResponse struct {
	Message string
	Error   error
	JUID    string
}

type JailsResponse struct {
	Message string
	Error   error
	Jails   []Jail
}

type JailResponse struct {
	Message string
	Error   error
	Jails   Jail
}

type JailStateResponse struct {
	Message string
	Error   error
	JailState   JailState
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

func validForm(bucketName []byte, reqForm JailConfig) error {
	err := JestDB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			encoded := bytes.NewReader(v)
			form := JailConfig{}
			err := json.NewDecoder(encoded).Decode(&form)
			if err != nil {
				log.Warn("Couldn't decode a key:", err)
			}

			switch {
			case form.Hostname == reqForm.Hostname:
				return fmt.Errorf("Hostname already in use: " + reqForm.Hostname + ".")
			case form.JailName == reqForm.JailName:
				return fmt.Errorf("JailConfig name already in use: " + reqForm.JailName + ".")
			case form.IPV4Addr == reqForm.IPV4Addr:
				return fmt.Errorf("IP address already in use: " + reqForm.IPV4Addr + ".")
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	templates := listAllTemplates()

	for j := range templates {
		if templates[j].Name == reqForm.Template {
			return nil
		}
	}

	return fmt.Errorf("Invalid template: " + reqForm.Template)
}

func CreateJailsEndpoint(w http.ResponseWriter, r *http.Request) {
	jUID := uuid.NewV4()

	var form JailConfig
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

	Defaults := JailConfig{
		`0`,
		`0`,
		`0`,
		`0`,
		`0`,
		`/var/log/jail_`+form.JailName+`_console.log`,
		form.Hostname,
		form.IPV4Addr,
		"root",
		form.JailName,
		filepath.Join(Conf.JestDir, form.JailName),
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

	if form.UseDefaults == true {
		encoded, err := json.Marshal(Defaults)
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

	/*
		JestDB.View(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucketName)
			v := b.Get(jUID.Bytes())

			json.NewDecoder(bytes.NewReader(v)).Decode(&form)
			return nil
		})
	*/

	// ToDo: We are basically validating the template twice, clean this up...
	template, _ := getTemplate(form.Template, listAllTemplates())
	fmt.Println("Template name:", template.ZFSParams.Name)
	snapshot, err := FindZFSSnapshot(template.ZFSParams.Name + "/." + template.Name)
	fmt.Println(snapshot)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := CreateJailResponse{"Couldn't find the snapshot.", err, jUID.String()}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error, "jUID": jUID.String()}).Warn(res.Message)
		return
	}

	fmt.Println("JestDir:", Conf.JestDir, "JestDataset:", Conf.JestDataset)

	opts := make(map[string]string)
	opts["mountpoint"] = filepath.Join(Conf.JestDir, form.JailName)
	if template.ZFSParams.Compression {
		opts["compression"] = "on"
	}

	_, err = CloneZFSSnapshot(snapshot, Conf.JestDataset+"/"+form.JailName, opts)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := CreateJailResponse{"Couldn't clone the template snapshot.", err, jUID.String()}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error, "jUID": jUID.String()}).Warn(res.Message)
		return
	}

	res := CreateJailResponse{"Jail created successfully", nil, jUID.String()}
	log.WithFields(log.Fields{"error": res.Error, "jUID": res.JUID}).Info(res.Message)
	json.NewEncoder(w).Encode(res)
	return
}

// ToDo: Add error handling here
func listAllJails() []Jail {
	var jailConfig = []JailConfig{}
	var jail = []Jail{}

	JestDB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			encoded := bytes.NewReader(v)
			form := JailConfig{}
			err := json.NewDecoder(encoded).Decode(&form)
			if err != nil {
				log.Warn("Couldn't decode a key:", err)
			}

			jailConfig = append(jailConfig, form)

		}
		return nil
	})

	for j := range jailConfig {
		jailStatus, _ := statusJail(jailConfig[j])
		jail = append(jail, Jail{jailConfig[j].JailName, jailConfig[j], jailStatus})
	}
	return jail
}

func returnJailConfig(name string) (JailConfig, error) {
	jails := listAllJails()

	for j := range jails {
		if jails[j].JailConfig.JailName == name {
			return jails[j].JailConfig, nil
		}
	}

	return JailConfig{}, fmt.Errorf("Couldn't find the jail "+name+".")
}

func ListJailsEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Info("Received a get jails request from " + r.RemoteAddr)
	HostNotInitialised(w, r)

	jails := listAllJails()

	if len(jails) < 1 {
		w.WriteHeader(http.StatusNotFound)
		res := JailsResponse{"No jails found.", fmt.Errorf("There are no jails enabled on this host."), jails}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	w.WriteHeader(http.StatusOK)
	res := JailsResponse{"Jails found.", nil, jails}
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
		res := JailResponse{"No jails found.", fmt.Errorf("There are no jails enabled on this host."), Jail{}}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	for j := range jails {
		if jails[j].JailConfig.JailName == vars["name"] {
			w.WriteHeader(http.StatusOK)
			res := JailResponse{"Jail found.", nil, jails[j]}
			log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
			json.NewEncoder(w).Encode(res)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	res := JailResponse{"Jail not found.", fmt.Errorf("There is no jail on this host with the name " + vars["name"]), Jail{}}
	log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
	json.NewEncoder(w).Encode(res)
	return
}

func startJail(jail JailConfig) (JailState, error) {
	cmd := `jail -c allow.raw_sockets="`+jail.AllowRawSockets+`"`+
		` allow.mount allow.set_hostname="`+jail.AllowSetHostname+`"`+
		` allow.sysvipc="`+jail.AllowSysVIPC+`"`+
		` exec.clean`+
		` exec.consolelog="`+jail.ConsoleLog+`"`+
		` host.hostname="`+jail.Hostname+`"`+
		` ip4.addr="`+jail.IPV4Addr+`"`+
		` exec.jail_user="`+jail.JailUser+`"`+
		` path="`+jail.Path+`"`+
		` exec.system_user="`+jail.SystemUser+`"`+
		` exec.start="`+jail.Start+`"`+
		` exec.stop="`+jail.Stop+`"`

	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		log.WithFields(log.Fields{"error": err, "command": cmd, "output": string(out)}).Warning("Command failed.")
		return JailState{}, err
	}

	jailStatus, err := statusJail(jail)
	return jailStatus, err
}

func stopJail(jail JailConfig) (JailState, error) {
	jID, err := getJID(jail.JailName)
	if err != nil {
		return JailState{}, err
	}

	cmd := `jail -r `+jID
	out, _ := exec.Command("sh", "-c", cmd).Output()

	if err != nil {
		log.WithFields(log.Fields{"error": err, "command": cmd, "output": string(out)}).Info("Jail not running.")
		return JailState{}, err
	}

	return statusJail(jail)
}

func statusJail(jail JailConfig) (JailState, error) {
	jid, err := getJID(jail.JailName)
	if err != nil {
		return JailState{jail.JailName, false, ""}, err
	}

	if jid == "" {
		return JailState{jail.JailName, false, ""}, nil
	}

	return JailState{jail.JailName, true, jid}, nil
}

func getJID(name string) (string, error) {
	cmd := `jls | awk '/`+name+`/{print $1}' | egrep -o "[0-9]*" | tr -d '\n'`
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		log.WithFields(log.Fields{"error": err, "command": cmd, "output": string(out)}).Warning("Command failed.")
		return "", err
	}

	return string(out), nil
}

func ChangeJailStateEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Info("Received a change jail state request from " + r.RemoteAddr)
	var form Jail
	HostNotInitialised(w, r)

	log.Debug("Decoding the JSON request.")
	err := json.NewDecoder(r.Body).Decode(&form)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		res := JailStateResponse{"Failed to decode the JSON request", err, JailState{}}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"request": form, "error": err}).Warn(res.Message)
		return
	}
	log.WithFields(log.Fields{"request": form}).Debug("Decoded JSON request.")

	jail, err := returnJailConfig(form.JailState.Name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := JailStateResponse{"Couldn't find the jail.", err, JailState{}}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	if form.JailState.Running == false {
		stopState, err := stopJail(jail)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			res := JailStateResponse{"Couldn't stop the jail.", err, JailState{}}
			log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
			json.NewEncoder(w).Encode(res)
			return
		}

		w.WriteHeader(http.StatusOK)
		res := JailStateResponse{"Jail stopped.", nil, stopState}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	startState, err := startJail(jail)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := JailResponse{"Couldn't start the jail.", err, Jail{}}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	w.WriteHeader(http.StatusOK)
	res := JailStateResponse{"Jail started.", nil, startState}
	log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
	json.NewEncoder(w).Encode(res)
	return
}

func DeleteJailEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Info("Received a change jail state request from " + r.RemoteAddr)
	vars := mux.Vars(r)
	jName := vars["name"]
	HostNotInitialised(w, r)

	err := JestDB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("jails"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			encoded := bytes.NewReader(v)
			form := JailConfig{}
			err := json.NewDecoder(encoded).Decode(&form)
			if err != nil {
				log.Warn("Couldn't decode a key:", err)
			}

			if form.JailName == jName {
				err := b.Delete(k)
				return err
			}
		}

		return fmt.Errorf("There are no jails with the name " + jName + " to delete.")
	})

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		res := JailResponse{"Couldn't delete jail.", err, Jail{}}
		log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
		json.NewEncoder(w).Encode(res)
		return
	}

	w.WriteHeader(http.StatusOK)
	res := JailResponse{"Jail deleted.", nil, Jail{}}
	log.WithFields(log.Fields{"error": res.Error}).Info(res.Message)
	json.NewEncoder(w).Encode(res)
	return
}
