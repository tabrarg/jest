package main

import (
	"encoding/json"
	"errors"
	"github.com/jlaffaye/ftp"
	"github.com/mholt/archiver"
	"github.com/mistifyio/go-zfs"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
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

type ZFSParams struct {
	BaseDataset string
	Mountpoint  string
	Compression bool
}

type FreeBSDParams struct {
	Version      string
	ApplyUpdates bool
}

const FTPSite = "ftp5.us.freebsd.org:21"

func initDataset(i InitCreate) ([]zfs.Dataset, error) {
	var datasets []zfs.Dataset

	rootOpts := make(map[string]string)
	rootOpts["mountpoint"] = i.ZFSParams.Mountpoint
	if i.ZFSParams.Compression {
		rootOpts["compression"] = "on"
	}
	log.WithFields(log.Fields{"volName": i.ZFSParams.BaseDataset, "params": rootOpts}).Debug("Creating dataset.")
	rootJailDataset, err := zfs.CreateFilesystem(i.ZFSParams.BaseDataset, rootOpts)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "volName": i.ZFSParams.BaseDataset}).Warning("Failed to create dataset")
		return datasets, err
	}
	//ToDo: Handle the error here and the other one below
	rootJailDataset.SetProperty("jest:name", "root")

	baseOpts := make(map[string]string)
	baseOpts["mountpoint"] = filepath.Join(i.ZFSParams.Mountpoint, "."+i.FreeBSDParams.Version)
	log.WithFields(log.Fields{"volName": i.ZFSParams.BaseDataset+"/."+i.FreeBSDParams.Version, "params": rootOpts}).Debug("Creating dataset.")
	baseJailDataset, err := zfs.CreateFilesystem(i.ZFSParams.BaseDataset+"/."+i.FreeBSDParams.Version, baseOpts)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "volName": i.ZFSParams.BaseDataset}).Warning("Failed to create dataset")
		return datasets, err
	}
	baseJailDataset.SetProperty("jest:name", "baseJail")

	datasets = append(datasets, *rootJailDataset, *baseJailDataset)
	return datasets, nil
}

func downloadVersion(ver string, path string, files []string) error {
	log.WithFields(log.Fields{"site": FTPSITE}).Debug("Connecting to FreeBSD FTP mirror.")
	client, err := ftp.Dial(FTPSite)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "site": FTPSITE}).Warning("Couldn't connect to the FreeBSD FTP mirror.")
		return err
	}

	log.WithFields(log.Fields{"site": FTPSITE, "credentials": "anonymous/anonymous"}).Debug("Logging in to FTP mirror.")
	err = client.Login("anonymous", "anonymous")
	if err != nil {
		log.WithFields(log.Fields{"error": err, "site": FTPSITE, "credentials": "anonymous/anonymous"}).Warning("Couldn't login to the FreeBSD FTP mirror.")
		return err
	}

	for i := 0; i < len(files); i++ {
		log.WithFields(log.Fields{"file": files[i]}).Debug("Creating file.")
		file, err := os.Create(filepath.Join(path, files[i]))
		if err != nil {
			log.WithFields(log.Fields{"error": err, "file": files[i]}).Warning("Couldn't create file.")
			return err
		}

		log.WithFields(log.Fields{"file": "pub/FreeBSD/releases/amd64/" + ver + "/" + files[i]}).Debug("Downloading file.")
		resp, err := client.Retr("pub/FreeBSD/releases/amd64/" + ver + "/" + files[i])
		if err != nil {
			log.WithFields(log.Fields{"error": err, "file": "pub/FreeBSD/releases/amd64/" + ver + "/" + files[i]}).Warning("Couldn't download file.")
			return err
		}

		_, err = io.Copy(file, resp)

		file.Close()
		resp.Close()
		log.WithFields(log.Fields{"file": files[i]}).Debug("Closed file.")
	}

	return nil
}

func validateVersion(v string) error {
	regex := `^[0-9]*\.[0-9]*-[A-Z0-9]*$`
	log.WithFields(log.Fields{"version": v, "regex": regex}).Debug("Validating FreeBSD version against the regex.")
	r, err := regexp.Compile(regex)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "version": v, "regex": regex}).Debug("Failed to compile the regex.")
		return err
	}

	if r.MatchString(v) == false {
		//toDo: Find out why this error doesn't get returned in our response
		log.WithFields(log.Fields{"version": v, "regex": regex}).Warning("Failed to match the version against the regex.")
		return errors.New("The version specified: " + v + " is not valid. The version should match the regex " + regex)
	}

	return nil
}

func extractVersion(path string, files []string) error {
	//ToDo: Extract these concurrently, this part takes ages
	for i := 0; i < len(files); i++ {
		log.WithFields(log.Fields{"file": files[i], "path": path}).Debug("Extracting FreeBSD archive file.")
		err := archiver.TarXZ.Open(filepath.Join(path, files[i]), path)
		if err != nil {
			log.WithFields(log.Fields{"error": err, "file": files[i], "path": path}).Debug("Couldn't extract the archive file.")
			return err
		}
	}

	return nil
}

func removeOldArchives(path string, files []string) error {
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

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

func prepareBaseJail(path string, applyUpdates bool) (string, error) {
	log.Debug("Copying /etc/resolv.conf into the base jail.")
	err := copyFile("/etc/resolv.conf", filepath.Join(path, "/etc/resolv.conf"))
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
	err = syscall.Chroot(path)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Couldn't chroot into the base jail.")
		return "", err
	}

	dirs := []string{"/usr/ports", "/usr/home."}
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
	//ToDo: This check doesn't seem to be working
	if _, err := os.Stat("/home"); err == nil {
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

	switch {
	case applyUpdates == true:
		cmds = append(cmds, `freebsd-update --not-running-from-cron fetch install`)
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

	log.Debug("Running the pkg command to generate directories.")
	_, err = exec.Command("sh", "-c", "pkg").Output()
	if err != nil {
		// Do nothing. We know pkg will spit out some errors, we just want it to create the dirs.
	}

	log.Debug("Exiting from the chroot.")
	err = syscall.Chroot(".")
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Couldn't exit from the chroot.")
		return "", err
	}

	/*
		switch {
		case applyUpdates == true:
			cmd := `freebsd-update fetch install -b ` + path
			log.WithFields(log.Fields{"command": cmd}).Debug("Updating base jail.")
			out, err := exec.Command("sh", "-c", cmd).Output()
			if err != nil {
				log.WithFields(log.Fields{"error": err, "command": cmd}).Warning("Command failed.")
				return "", err
			}
			log.WithFields(log.Fields{"output": string(out), "command": cmd}).Debug("Finished command.")
		}
	*/
	return pw, nil
}

func CreateInitEndpoint(w http.ResponseWriter, r *http.Request) {
	var i InitCreate
	var datasets []zfs.Dataset
	files := []string{"base.txz", "lib32.txz", "src.txz"}
	log.Info("Received a initialisation request.")

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
	err = validateVersion(i.FreeBSDParams.Version)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		res := InitResponse{"Invalid FreeBSD Version specified.", err, datasets, ""}
		json.NewEncoder(w).Encode(res)
		log.WithFields(log.Fields{"Error": err}).Warn(res.Message)
		return
	}

	log.Info("Creating ZFS datasets.")
	datasets, err = initDataset(i)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(InitResponse{"Failed to create dataset " + i.ZFSParams.BaseDataset + ".", err, datasets, ""})
		return
	}
	log.WithFields(log.Fields{"request": i, "datasets": datasets}).Info("Created ZFS datasets.")

	log.Info("Downloading FreeBSD files.")
	err = downloadVersion(i.FreeBSDParams.Version, filepath.Join(i.ZFSParams.Mountpoint, "."+i.FreeBSDParams.Version), files)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(InitResponse{"Failed to get FreeBSD files for version " + i.FreeBSDParams.Version + ".", err, datasets, ""})
		return
	}

	log.Info("Extracting FreeBSD archive files.")
	err = extractVersion(filepath.Join(i.ZFSParams.Mountpoint, "."+i.FreeBSDParams.Version), files)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(InitResponse{"Failed to extract FreeBSD archive files.", err, datasets, ""})
		return
	}

	log.Info("Removing the extracted FreeBSD archive files.")
	err = removeOldArchives(filepath.Join(i.ZFSParams.Mountpoint, "."+i.FreeBSDParams.Version), files)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(InitResponse{"Failed to cleanup the extracted FreeBSD files.", err, datasets, ""})
		return
	}

	log.Info("Preparing the FreeBSD base jail.")
	pw, err := prepareBaseJail(filepath.Join(i.ZFSParams.Mountpoint, "."+i.FreeBSDParams.Version), i.FreeBSDParams.ApplyUpdates)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(InitResponse{"Failed to prepare the base jail.", err, datasets, ""})
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(InitResponse{"Successfully initialised the host for use with Jest.", nil, datasets, pw})
	log.Info("Successfully finished initialising the host for use with Jest.")
}

func GetInitEndpoint(w http.ResponseWriter, r *http.Request) {
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
		w.WriteHeader(http.StatusNoContent)
		json.NewEncoder(w).Encode(InitResponse{
			"Failed to find any ZFS datasets registered with Jest.",
			errors.New("No ZFS datasets containing property jest:name found"),
			datasets,
			"",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(InitResponse{"Successfully found Jest datasets.", nil, datasets, ""})
}

func DeleteInitEndpoint(w http.ResponseWriter, r *http.Request) {

}
