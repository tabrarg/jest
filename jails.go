package main

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"
	"errors"
)

type CreateJailForm struct {
	AllowRawSockets  bool
	AllowMount       bool
	AllowSetHostname bool
	AllowSysVIPC     bool
	Clean            bool
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
}

type CreateJailResponse struct {
	Message string
	Error   error
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

func checkJailName(name string) (bool, error) {
	exists, err := CheckFileForString("/etc/jail.conf", name+` {`)
	if err != nil {
		return false, err
	}

	if exists == false {
		return false, nil
	}

	return true, nil
}

func writeJailConfig(jconf CreateJailForm) error {
	err := AppendStringToFile("/etc/jail.conf", jconf.JailName+" {\n")
	if err != nil {
		//ToDo: Do something
	}
	//AppendStringToFile("/etc/jail.conf", AllowRawSocketsLine+jconf)
	return nil
}

func CreateJailsEndpoint(w http.ResponseWriter, r *http.Request) {
	var form CreateJailForm
	log.Info("Received a create jail request from " + r.RemoteAddr)

	log.Debug("Decoding the JSON request.")
	err := json.NewDecoder(r.Body).Decode(&form)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"Failed to decode the JSON request", err}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"request": form, "error": err}).Warn(res.Message)
		return
	}
	log.WithFields(log.Fields{"request": form}).Debug("Decoded JSON request.")

	jailNameExists, err := checkJailName(form.JailName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := CreateJailResponse{"Failed to validate the jail name.", err}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": err}).Warn(res.Message)
	}

	switch {
	case jailNameExists == true:
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"Jail name already in use.", errors.New("This jail name was detected in /etc/jail.conf, please use a different one.")}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error}).Warn(res.Message)
	case form.JailName == "":
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"No jail name supplied.", errors.New("You must supply a jail name to be used.")}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error}).Warn(res.Message)
	case form.Template == "":
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"No template supplied.", errors.New("You must include a template with the request, the template is the name of the base jail you wish to clone.")}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error}).Warn(res.Message)
	case form.Hostname == "":
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"No hostname supplied.", errors.New("You must include a hostname with the request.")}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error}).Warn(res.Message)
	case form.IPV4Addr == "":
		w.WriteHeader(http.StatusNotAcceptable)
		res := CreateJailResponse{"No IP address supplied.", errors.New("You must include a IP with the request.")}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": res.Error}).Warn(res.Message)
	}

	if form.UseDefaults == true {
		defaults := CreateJailForm{
			true,
			true,
			true,
			true,
			true,
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

		err := writeJailConfig(defaults)
		_ = err
	}
	/*
		- Check if name is available
		- Check if UseDefaults is set
			- Check if Template is set
				- Create Jail
		- Check if template is set
			- Create Jail

	*/
}
