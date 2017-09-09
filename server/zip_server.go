package main

import (
 "fmt"
 "log"
 "os"
 "io"
 "archive/zip"
 "path/filepath"
 "strings"
 "time"
 "gopkg.in/mgo.v2"
)

const (
	INTERVAL_PERIOD time.Duration = 24 * time.Hour
	HOUR_TO_TICK int = 23
	MINUTE_TO_TICK int = 59
	SECOND_TO_TICK int = 30
)

func zipit(source, target string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
	return err
}

func unzip(archive, target string) error {
    reader, err := zip.OpenReader(archive)
    if err != nil {
        return err
    }

    if err := os.MkdirAll(target, 0755); err != nil {
        return err
    }

    for _, file := range reader.File {
        path := filepath.Join(target, file.Name)
        if file.FileInfo().IsDir() {
            os.MkdirAll(path, file.Mode())
            continue
        }

        fileReader, err := file.Open()
        if err != nil {
            if fileReader != nil {
                fileReader.Close()
            }
            return err
        }

        targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
        if err != nil {
            fileReader.Close()
            if targetFile != nil {
                targetFile.Close()
            }
            return err
        }

        if _, err := io.Copy(targetFile, fileReader); err != nil {
            fileReader.Close()
            targetFile.Close()
            return err
        }

        fileReader.Close()
        targetFile.Close()
    }

    return nil
}

func zipDayTickets(dayName string) {
	os.Chdir(fmt.Sprintf("./tickets/%s", dayName))
	zipit(".", fmt.Sprintf("../%s.zip", dayName))
	os.Chdir("../..")
}

func updateTicker() *time.Ticker {
	mNow := time.Now()
	nextTick := time.Date(mNow.Year(), mNow.Month(), mNow.Day(), HOUR_TO_TICK, MINUTE_TO_TICK, SECOND_TO_TICK, 0, time.Local)
	if !nextTick.After(time.Now()) {
		nextTick = nextTick.Add(INTERVAL_PERIOD)
	}
	fmt.Println(nextTick, "- next run")
	diff := nextTick.Sub(time.Now())
	return time.NewTicker(diff)
}

////////// MongoDB //////////
func ensureIndex(s *mgo.Session) {
	session := s.Copy()
	defer session.Close()
	c := session.DB("polices").C("policeanchordocs")
	index := mgo.Index{
		Key:        []string{"userid"},
		Unique:     false,
		DropDups:   false,
		Background: true,
		Sparse:     true,
	}
	err := c.EnsureIndex(index)
	if err != nil {
		panic(err)
	}

}

func removeAnchors() {

	/////////////////////////////////////////////////
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer session.Close()

	session.SetMode(mgo.Monotonic, true)
	ensureIndex(session)

	_, err = session.DB("polices").C("policeanchordocs").RemoveAll(nil)
	if err != nil {
		log.Println("Failed to remove anchors: ", err)
	}

}


func main() {
	// zipDayTickets("2017-8-6")
	ticker := updateTicker()
    for {
		<-ticker.C
		mNow := time.Now()
		folderName := fmt.Sprintf("%d-%d-%d", mNow.Year(), int(mNow.Month()), mNow.Day())

		removeAnchors()
		fmt.Println(time.Now(), "- just removed anchors")


		zipDayTickets(folderName)
		fmt.Println(time.Now(), fmt.Sprintf("- just zipped %s", folderName))
		ticker = updateTicker()
    }
}
