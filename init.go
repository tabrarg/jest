package main

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/jlaffaye/ftp"
	"github.com/mistifyio/go-zfs"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type InitResponse struct {
	Message  string
	Error    error
	Datasets []zfs.Dataset
	Password string
}

type InitCreate struct {
	ZFSParams     ZFSParams
	FreeBSDParams FreeBSDParams
}

// ToDo: This should be a list of map[string]string really, so people can set any properties they like
type ZFSParams struct {
	Name        string
	Mountpoint  string
	Compression bool
}

type FreeBSDParams struct {
	Name         string
	Version      string
	ApplyUpdates bool
}

//ToDo: Something better than this:
const FTPSite = "ftp5.us.freebsd.org:21"

func InitDataset(i InitCreate) ([]zfs.Dataset, error) {
	var datasets []zfs.Dataset

	rootOpts := make(map[string]string)
	rootOpts["mountpoint"] = i.ZFSParams.Mountpoint
	if i.ZFSParams.Compression {
		rootOpts["compression"] = "on"
	}
	rootJailDataset, err := CreateZFSDataset(i.ZFSParams.Name, rootOpts)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "filesystem": i.ZFSParams.Name}).Warning("Failed to create dataset")
		return datasets, err
	}

	jestOpts := map[string]string{"mountpoint": filepath.Join(i.ZFSParams.Mountpoint, ".jest")}
	jestDataset, err := CreateZFSDataset(i.ZFSParams.Name+"/.jest", jestOpts)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "filesystem": i.ZFSParams.Name}).Warning("Failed to create dataset")
		return datasets, err
	}

	baseOpts := map[string]string{"mountpoint": filepath.Join(i.ZFSParams.Mountpoint, "."+i.FreeBSDParams.Name)}
	baseJailDataset, err := CreateZFSDataset(i.ZFSParams.Name+"/."+i.FreeBSDParams.Name, baseOpts)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "filesystem": i.ZFSParams.Name}).Warning("Failed to create dataset")
		return datasets, err
	}

	//ToDo: Handle the error here properly
	err = rootJailDataset.SetProperty("jest:dir", filepath.Join(i.ZFSParams.Mountpoint, "/.jest"))
	if err != nil {
		log.Warn(err)
	}

	datasets = append(datasets, *rootJailDataset, *baseJailDataset, *jestDataset)
	return datasets, nil
}

func DownloadVersion(ver string, path string, files []string) error {
	log.WithFields(log.Fields{"site": FTPSite}).Debug("Connecting to FreeBSD FTP mirror.")
	client, err := ftp.Dial(FTPSite)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "site": FTPSite}).Warning("Couldn't connect to the FreeBSD FTP mirror.")
		return err
	}

	log.WithFields(log.Fields{"site": FTPSite, "credentials": "anonymous/anonymous"}).Debug("Logging in to FTP mirror.")
	err = client.Login("anonymous", "anonymous")
	if err != nil {
		log.WithFields(log.Fields{"error": err, "site": FTPSite, "credentials": "anonymous/anonymous"}).Warning("Couldn't login to the FreeBSD FTP mirror.")
		return err
	}

	for i := 0; i < len(files); i++ {
		log.WithFields(log.Fields{"file": files[i]}).Debug("Creating file.")
		file, err := os.Create(filepath.Join(path, files[i]))
		if err != nil {
			log.WithFields(log.Fields{"error": err, "file": files[i]}).Warning("Couldn't create file.")
			return err
		}

		log.WithFields(log.Fields{"file": FTPSite + "/pub/FreeBSD/releases/amd64/" + ver + "/" + files[i]}).Debug("Downloading file.")
		resp, err := client.Retr("pub/FreeBSD/releases/amd64/" + ver + "/" + files[i])
		if err != nil {
			log.WithFields(log.Fields{"error": err, "file": FTPSite + "/pub/FreeBSD/releases/amd64/" + ver + "/" + files[i]}).Warning("Couldn't download file.")
			return err
		}

		_, err = io.Copy(file, resp)

		file.Close()
		resp.Close()
		log.WithFields(log.Fields{"file": files[i]}).Debug("Closed file.")
	}

	return nil
}

func ValidateVersion(v string) error {
	regex := `^[0-9]*\.[0-9]*-[A-Z0-9]*$`
	log.WithFields(log.Fields{"version": v, "regex": regex}).Debug("Validating FreeBSD version against the regex.")
	r, err := regexp.Compile(regex)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "version": v, "regex": regex}).Debug("Failed to compile the regex.")
		return err
	}

	if r.MatchString(v) == false {
		log.WithFields(log.Fields{"version": v, "regex": regex}).Warning("Failed to match the version against the regex.")
		return fmt.Errorf("The version specified: " + v + " is not valid. The version should match the regex " + regex)
	}

	return nil
}

func RemoveOldArchives(path string, files []string) error {
	for i := 0; i < len(files); i++ {
		log.WithFields(log.Fields{"file": files[i], "path": path}).Debug("Removing old FreeBSD archive file.")
		err := os.Remove(filepath.Join(path, files[i]))
		if err != nil {
			log.WithFields(log.Fields{"error": err, "file": files[i], "path": path}).Warning("Couldn't remove the old archive file.")
			return err
		}
	}

	return nil
}

func PrepareBaseJail(path string, applyUpdates bool) (string, error) {
	log.Debug("Copying /etc/resolv.conf into the base jail.")
	err := CopyFile("/etc/resolv.conf", filepath.Join(path, "/etc/resolv.conf"))
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Failed to copy /etc/resolv.conf into the base jail.")
		return "", err
	}

	log.Debug("Creating /dev/null volume in base jail")
	baseDev := filepath.Join(path, "/dev")
	baseDevNull := filepath.Join(baseDev, "/null")
	if _, err := os.Stat(baseDevNull); err == nil {
		log.WithFields(log.Fields{"volName": baseDevNull}).Debug("Volume already exists - skipping.")
	} else {
		cmd := "mount -t devfs dev " + baseDev
		log.WithFields(log.Fields{"volName": baseDev}).Debug("Making " + baseDev + " volume.")
		out, err := exec.Command("sh", "-c", cmd).Output()
		if err != nil {
			log.WithFields(log.Fields{"error": err, "command": cmd, "output": string(out)}).Warning("Command failed.")
			return "", err
		}
	}

	log.Debug("Chrooting into the base jail.")
	err = os.Chdir("/")
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Couldn't cd into /")
		return "", err
	}
	_, err = exec.Command("sh", "-c", "sysctl kern.chroot_allow_open_directories=2").Output()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Couldn't set sysctl kern.chroot_allow_open_directories to allow us to escape chroot")
		return "", err
	}
	exitChroot, err := Chroot(path)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Couldn't chroot into the base jail.")
		return "", err
	}

	dirs := []string{"/usr/ports", "/usr/home"}
	for i := 0; i < len(dirs); i++ {
		log.WithFields(log.Fields{"dirName": dirs[i]}).Debug("Checking directory.")
		if _, err := os.Stat(dirs[i]); err == nil {
			log.WithFields(log.Fields{"dirName": dirs[i]}).Debug("Directory already exists - skipping.")
		} else {
			err := os.Mkdir(dirs[i], 755)
			if err != nil {
				log.WithFields(log.Fields{"error": err}).Warning("Could create the directory " + dirs[i] + ".")
				return "", err
			}
			log.WithFields(log.Fields{"dirName": dirs[i]}).Debug("Created directory.")
		}
	}

	log.Debug("Linking /usr/home to /home.")
	_, err = os.Stat("/home")
	if err == nil {
		log.WithFields(log.Fields{"linkName": "/home"}).Debug("Symlink already exists - skipping.")
	} else {
		err = os.Symlink("/usr/home", "/home")
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Warning("Couldn't link /usr/home to /home.")
			return "", err
		}
	}

	log.Debug("Changing directory to /etc/mail.")
	err = os.Chdir("/etc/mail")
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Couldn't change into /etc/mail directory.")
		return "", err
	}

	cmds := []string{`make aliases`}

	//ToDo: Rip all this out and use the functions in utils for finding stings and appending them

	rcConfOpts := []string{
		`"#Added by Jest:\n"`,
		`"sendmail_enable=\"NONE\"\n"`,
		`"syslogd_flags=\"-ss\"\n"`,
		`"rpcbind_enable=\"NO\"\n"`,
	}
	for i := 0; i < len(rcConfOpts); i++ {
		f, err := ioutil.ReadFile("/etc/rc.conf")
		if err != nil {
			log.WithFields(log.Fields{"fileName": "/etc/rc.conf"}).Warning("/etc/rc.conf doesn't exist - creating it.")
			_, err = os.Create("/etc/rc.conf")
			if err != nil {
				log.WithFields(log.Fields{"error": err}).Warning("Couldn't create /etc/rc.conf.")
				return "", err
			}
		}
		s := string(f)
		if strings.Contains(s, rcConfOpts[i]) == true {
			log.WithFields(log.Fields{"Option": `printf ` + rcConfOpts[i] + ` >> /etc/make.conf`}).Debug("Line already exists in file - skipping.")
		} else {
			log.WithFields(log.Fields{"Option": `printf ` + rcConfOpts[i] + ` >> /etc/rc.conf`}).Debug("Adding command to list.")
			cmds = append(cmds, `printf `+rcConfOpts[i]+` >> /etc/rc.conf`)
		}
	}

	makeConfOpts := []string{
		`"#Added by Jest:\n"`,
		`"WITH_PKGNG=yes\n"`,
		`"WRKDIRPREFIX=/var/ports\n"`,
		`"DISTDIR=/var/ports/distfiles\n"`,
		`"PACKAGES=/var/ports/packages\n"`,
		`"INDEXDIR=/usr/ports\n"`,
	}
	for i := 0; i < len(makeConfOpts); i++ {
		f, err := ioutil.ReadFile("/etc/make.conf")
		if err != nil {
			log.WithFields(log.Fields{"fileName": "/etc/make.conf"}).Warning("/etc/make.conf doesn't exist - creating it.")
			_, err = os.Create("/etc/make.conf")
			if err != nil {
				log.WithFields(log.Fields{"error": err}).Warning("Couldn't create /etc/make.conf.")
				return "", err
			}
		}
		s := string(f)
		if strings.Contains(s, makeConfOpts[i]) == true {
			log.WithFields(log.Fields{"Option": `printf ` + makeConfOpts[i] + ` >> /etc/make.conf`}).Debug("Line already exists in file - skipping.")
		} else {
			log.WithFields(log.Fields{"Option": `printf ` + makeConfOpts[i] + ` >> /etc/make.conf`}).Debug("Adding command to list.")
			cmds = append(cmds, `printf `+makeConfOpts[i]+` >> /etc/make.conf`)
		}
	}

	pw := RandomString(128)
	cmds = append(cmds, `echo "`+pw+`" | pw usermod root -h 0`)

	for i := 0; i < len(cmds); i++ {
		log.WithFields(log.Fields{"command": cmds[i]}).Debug("Executing command.")
		out, err := exec.Command("sh", "-c", cmds[i]).Output()
		if err != nil {
			log.WithFields(log.Fields{"error": err, "command": cmds[i], "output": string(out)}).Warning("Command failed.")
			return "", err
		}
		log.WithFields(log.Fields{"output": string(out), "command": cmds[i]}).Debug("Finished command.")
	}

	ignoreErrorCmds := []string{`pkg`}
	switch {
	case applyUpdates == true:
		ignoreErrorCmds = append(ignoreErrorCmds, `freebsd-update --not-running-from-cron fetch install`)
	}
	log.Debug("Updating the base jail.")
	for i := 0; i < len(ignoreErrorCmds); i++ {
		_, err = exec.Command("sh", "-c", ignoreErrorCmds[i]).Output()
		if err != nil {
			// Do nothing. We know pkg will spit out some errors, we just want it to create the dirs.
		}
	}

	log.Debug("Exiting from the chroot.")
	err = exitChroot()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Couldn't exit from the chroot.")
		return "", err
	}

	_, err = exec.Command("sh", "-c", "sysctl kern.chroot_allow_open_directories=1").Output()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Couldn't set sysctl kern.chroot_allow_open_directories to restrict croot access")
		return "", err
	}

	return pw, nil
}

func prepareHostConfig() error {
	exists, err := CheckFileForString("/etc/rc.conf", `jail_enable="YES"`)

	if exists == false {
		log.WithFields(log.Fields{"fileName": "/etc/rc.conf"}).Debug(`Adding jail_enable="YES" to /etc/rc.conf`)
		err = AppendStringToFile("/etc/rc.conf", "jail_enable=\"YES\"\n")
		if err != nil {
			log.WithFields(log.Fields{"fileName": "/etc/rc.conf", "error": err}).Warning("Failed to append the line to the config file.")
			return err
		}
	}

	return nil
}

func CreateInitEndpoint(w http.ResponseWriter, r *http.Request) {
	var i InitCreate
	var datasets []zfs.Dataset

	files := []string{"base.txz", "lib32.txz", "src.txz"}
	log.Info("Received a initialisation request from " + r.RemoteAddr)

	log.Debug("Checking if server is already initialised.")
	if IsInitialised == true {
		err := fmt.Errorf("This host is already initialised.")
		res := InitResponse{"Cannot initialise", err, datasets, ""}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"error": err}).Warn(res.Message)
	}

	log.Info("Decoding the JSON request.")
	err := json.NewDecoder(r.Body).Decode(&i)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		res := InitResponse{"Failed to decode JSON request.", err, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"request": i, "error": err}).Warn(res.Message)
		return
	}
	log.WithFields(log.Fields{"request": i}).Info("Decoded JSON request.")

	log.WithFields(log.Fields{"version": i.FreeBSDParams.Version}).Info("Validating FreeBSD version.")
	err = ValidateVersion(i.FreeBSDParams.Version)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		res := InitResponse{"Invalid FreeBSD Version specified.", err, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
		return
	}

	templatePath := filepath.Join(i.ZFSParams.Mountpoint, "."+i.FreeBSDParams.Name)

	log.Info("Creating ZFS datasets.")
	datasets, err = InitDataset(i)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := InitResponse{"Failed to create dataset " + i.ZFSParams.Name + ".", err, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
		return
	}
	log.WithFields(log.Fields{"request": i, "datasets": datasets}).Info("Created ZFS datasets.")

	log.Info("Downloading FreeBSD files.")
	err = DownloadVersion(i.FreeBSDParams.Version, filepath.Join(templatePath), files)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := InitResponse{"Failed to get FreeBSD files for version " + i.FreeBSDParams.Version + ".", err, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
		return
	}

	log.Info("Extracting FreeBSD archive files.")
	err = ExtractFiles(templatePath, files)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := InitResponse{"Failed to extract FreeBSD archive files.", err, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
		return
	}

	log.Info("Removing the extracted archive files.")
	err = RemoveOldArchives(templatePath, files)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := InitResponse{"Failed to cleanup the extracted FreeBSD files.", err, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
		return
	}

	log.Info("Preparing the base jail.")
	pw, err := PrepareBaseJail(templatePath, i.FreeBSDParams.ApplyUpdates)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := InitResponse{"Failed to prepare the base jail.", err, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
		return
	}

	// ToDo: Add error handling here if we can't find the jail
	log.Info("Taking a snapshot of the base jail.")
	for i := range datasets {
		if datasets[i].Mountpoint == templatePath {
			_, err := SnapshotZFSDataset(datasets[i])
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				res := InitResponse{"Failed to snapshot the base jail", err, datasets, ""}
				json.NewEncoder(w).Encode(res)
				log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
				return
			}
		}
	}

	log.Info("Preparing the host to run jails.")
	err = prepareHostConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := InitResponse{"Failed while preparing the host configuration files for jails.", err, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
		return
	}

	log.Info("Initialising host..")
	jestDir, isInitialised, initErr = InitStatus()
	JestDir = jestDir
	IsInitialised = isInitialised
	if initErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := InitResponse{"Failed while trying to find the created ZFS pool.", initErr, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
		return
	}

	tUID := uuid.NewV4()
	template := Template{i.FreeBSDParams.Name, false, templatePath, i.FreeBSDParams.Version, i.ZFSParams}

	log.Info("Writing template settings to the DB.")
	encoded, err := json.Marshal(template)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "tUID": tUID.String()}).Warn("Failed to encode the struct to JSON before writing to the JestDB.")
	}
	JestDB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("templates"))
		err := b.Put(tUID.Bytes(), encoded)
		return err
	})

	cUID := uuid.NewV4()
	log.Info("Writing Jest config to the DB.")
	config := Config{i.ZFSParams.Mountpoint, i.ZFSParams.Name, false}
	encoded, err = json.Marshal(config)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warn("Failed to encode the struct to JSON before writing to the JestDB.")
	}
	JestDB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		err := b.Put(cUID.Bytes(), encoded)
		return err
	})

	conf, err := LoadConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		res := InitResponse{"Failed trying to load the config from the DB.", err, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
		return
	}
	Conf = conf

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(InitResponse{"Successfully initialised the host for use with Jest.", nil, datasets, pw})
	log.Info("Successfully finished initialising the host for use with Jest.")
}

func GetInitEndpoint(w http.ResponseWriter, r *http.Request) {
	_ = r
	var datasets []zfs.Dataset

	l, err := zfs.ListZpools()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(InitResponse{"Failed to list list Zpools on the system.", err, datasets, ""})
		return
	}

	for i := range l {
		d, _ := l[i].Datasets()
		for a := range d {
			jestName, _ := d[a].GetProperty("jest:name")
			if jestName != "-" {
				datasets = append(datasets, *d[a])
			}
		}
	}

	if len(datasets) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(InitResponse{
			"Failed to find any ZFS datasets registered with Jest.",
			fmt.Errorf("No ZFS datasets containing property jest:name found"),
			datasets,
			"",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(InitResponse{"This server has been initialised for Jest.", nil, datasets, ""})
}

func DeleteInitEndpoint(w http.ResponseWriter, r *http.Request) {

}
