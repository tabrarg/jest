package main

import (
	log "github.com/sirupsen/logrus"
	"net/http"
	"path/filepath"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/boltdb/bolt"
	"encoding/json"
	"errors"
)

type CreateJailForm struct {
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
	/*
	exists, err := CheckFileForString("/etc/jail.conf", name+` {`)
	if err != nil {
		return false, err
	}

	if exists == false {
		return false, nil
	}

*/
	return false, nil
}

func writeJailConfig(jconf CreateJailForm, path string) error {
	err := AppendStringToFile(filepath.Join(path, jconf.JailName+".conf"), jconf.JailName+" {\n")
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

		if form.UseDefaults == "true" {
			defaults := CreateJailForm{
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

			// Write the jail config to individual files in $base/.config, build the whole file in memory and flush to disk.
			// Determine the base by setting a jest:Type field where one type is 'base' the others are 'template' and 'jail'
			// Set the jail config file path as a dataset property jest:JailConf = /usr/jail/.conf/example.conf
			// Generate a uid for each jail, so that name changes become just a ZFS property change
			// Store password as property jest:Token = $randomString (only for templates)

			jUID := uuid.NewV4()

			db, err := OpenDB()
			defer db.Close()
			if err != nil {
				log.Warn(err)
			}

			err = db.Update(func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket(jUID.Bytes())
				if err != nil {
					return fmt.Errorf("create bucket: %s", err)
				}
				return nil
			})
			if err != nil {
				panic(err)
			}

			if form.UseDefaults == "true" {
				db.Update(func(tx *bolt.Tx) error {
					b := tx.Bucket(jUID.Bytes())
					err := b.Put([]byte("AllowRawSockets"), []byte(defaults.AllowRawSockets))
					err = b.Put([]byte("AllowMount"), []byte(defaults.AllowMount))
					err = b.Put([]byte("AllowSetHostname"), []byte(defaults.AllowSetHostname))
					err = b.Put([]byte("AllowSysVIPC"), []byte(defaults.AllowSysVIPC))
					err = b.Put([]byte("Clean"), []byte(defaults.Clean))
					err = b.Put([]byte("ConsoleLog"), []byte(defaults.ConsoleLog))
					err = b.Put([]byte("Hostname"), []byte(defaults.Hostname))
					err = b.Put([]byte("IPV4Addr"), []byte(defaults.IPV4Addr))
					err = b.Put([]byte("JailUser"), []byte(defaults.JailUser))
					err = b.Put([]byte("JailName"), []byte(defaults.JailName))
					err = b.Put([]byte("Path"), []byte(defaults.Path))
					err = b.Put([]byte("SystemUser"), []byte(defaults.SystemUser))
					err = b.Put([]byte("Start"), []byte(defaults.Start))
					err = b.Put([]byte("Stop"), []byte(defaults.Stop))
					err = b.Put([]byte("Template"), []byte(defaults.Template))
					err = b.Put([]byte("UseDefaults"), []byte(defaults.UseDefaults))
					return err
				})
			} else {
				db.Update(func(tx *bolt.Tx) error {
					b := tx.Bucket([]byte(jUID.Bytes()))
					err := b.Put([]byte("AllowRawSockets"), []byte(form.AllowRawSockets))
					err = b.Put([]byte("AllowMount"), []byte(form.AllowMount))
					err = b.Put([]byte("AllowSetHostname"), []byte(form.AllowSetHostname))
					err = b.Put([]byte("AllowSysVIPC"), []byte(form.AllowSysVIPC))
					err = b.Put([]byte("Clean"), []byte(form.Clean))
					err = b.Put([]byte("ConsoleLog"), []byte(form.ConsoleLog))
					err = b.Put([]byte("Hostname"), []byte(form.Hostname))
					err = b.Put([]byte("IPV4Addr"), []byte(form.IPV4Addr))
					err = b.Put([]byte("JailUser"), []byte(form.JailUser))
					err = b.Put([]byte("JailName"), []byte(form.JailName))
					err = b.Put([]byte("Path"), []byte(form.Path))
					err = b.Put([]byte("SystemUser"), []byte(form.SystemUser))
					err = b.Put([]byte("Start"), []byte(form.Start))
					err = b.Put([]byte("Stop"), []byte(form.Stop))
					err = b.Put([]byte("Template"), []byte(form.Template))
					err = b.Put([]byte("UseDefaults"), []byte(form.UseDefaults))
					return err
				})
			}

			db.View(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(jUID.Bytes()))
				v := b.Get([]byte("JailName"))
				fmt.Printf("The answer is: %s\n", v)
				return nil
			})
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
