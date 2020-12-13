package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const defaultConfigFile = "config.json"

type ramdisk struct {
	Location string
	Size     string
}

type storage struct {
	From string
	To   string
}

var config struct {
	RAMDisks []ramdisk
	Load     []storage
	Save     []storage
	Period   int
	ChownUID int `json:"chown_uid"`
	ChownGID int `json:"chown_gid"`
	User     string
	Execute  string
}

func readConfig(filename string) error {
	log.Printf("Reading config ('%s')", filename)

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	configJSON, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	err = json.Unmarshal(configJSON, &config)
	if err != nil {
		return err
	}

	return nil
}

func mountRAMDisks() error {
	log.Printf("Mounting ramdisks")

	for _, ramdisk := range config.RAMDisks {
		err := os.MkdirAll(ramdisk.Location, 0755)
		if err != nil {
			return err
		}

		cmd := exec.Command("mount", "-t", "tmpfs", "-o", "size="+ramdisk.Size, "tmpfs", ramdisk.Location)
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func load() error {
	log.Printf("Loading")

	for _, load := range config.Load {
		cmd := exec.Command("sh", "-c", "cp -r "+load.From+" "+load.To)
		err := cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func chown() error {
	log.Printf("Changing owner")

	for _, ramdisk := range config.RAMDisks {
		cmd := exec.Command("chown", "-R", strconv.Itoa(config.ChownUID)+":"+strconv.Itoa(config.ChownGID), ramdisk.Location)
		err := cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func execute() error {
	log.Printf("Starting")

	cmd := exec.Command("sudo", "-u", config.User, config.Execute)
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func save() error {
	log.Printf("Saving")

	for _, save := range config.Save {
		cmd := exec.Command("sh", "-c", "cp -r "+save.From+" "+save.To)
		err := cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func unmountRAMDisks() error {
	log.Printf("Unmounting ramdisks")

	for _, ramdisk := range config.RAMDisks {
		cmd := exec.Command("umount", ramdisk.Location)
		err := cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func run() error {
	err := mountRAMDisks()
	if err != nil {
		return err
	}
	err = load()
	if err != nil {
		return err
	}
	err = chown()
	if err != nil {
		return err
	}

	running := true
	go func() {
		err = execute()
		if err != nil {
			log.Printf("Failed to execute\n%v", err)
		}
		running = false
	}()

	for running {
		err = save()
		if err != nil {
			log.Printf("Save failed\n%v", err)
		}
		time.Sleep(time.Duration(config.Period) * time.Second)
	}

	err = save()
	if err == nil {
		unmountRAMDisks()
		return nil
	}

	log.Printf("Final save failed, won't unmount ramdisks\n%v", err)
	return err
}

func main() {
	if len(os.Args) > 1 {
		err := readConfig(os.Args[1])
		if err != nil {
			panic(err)
		}
	} else {
		err := readConfig(defaultConfigFile)
		if err != nil {
			panic(err)
		}
	}

	err := run()
	if err != nil {
		log.Print(err)
	}
}
