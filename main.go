package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"html/template"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nfnt/resize"
)

var serveDir string

type LoggerResponseWriter struct {
	http.ResponseWriter
	code int
}

type Image struct {
	Name  string
	Thumb string
	URL   string
}

type ImagesPage struct {
	PageTitle string
	Images    []Image
}

func (lrw *LoggerResponseWriter) WriteHeader(code int) {
	lrw.code = code
	lrw.ResponseWriter.WriteHeader(code)
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// function logger logs all requests
func logger(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &LoggerResponseWriter{ResponseWriter: w, code: -1}
		handler.ServeHTTP(lrw, r)
		log.Printf("%d %s %s %s", lrw.code, r.Method, r.RemoteAddr, r.URL)
	})
}

func imageList(w http.ResponseWriter, r *http.Request) {
	const tpl = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.PageTitle}}</title>
    <style>
		* {
			padding: 0;
			margin: 0;
			border: 0;
		}
		html, body {
			font-family: arial;
			font-size: 16px;
			background: #fff;
			color: #aaa;
			text-align:center;
		}
		a {
			text-decoration: none;
		}
		.container {
			display: flex;
			flex-flow: row wrap;
		}
{{range .Images}}
		.popup-{{.Name}} {
			background-image: url("{{.URL}}");
			background-color: #000000;
			max-width: 100%;
			max-height: 100%;
			background-position: center;
			background-repeat: no-repeat;
			background-size: contain;
			position: fixed;
			z-index: 999;
			display: none;
		}
		.popup-{{.Name}}:target {
			outline: none;
			width: 100%;
			height: 100%;
			display: block !important;
		}
{{end}}
	</style>
</head>
<body>
{{range .Images}}
    <a href="#_">
        <div class="popup-{{.Name}}" id="{{.Name}}"></div>
    </a>
{{end}}
    <div class="container">
{{range .Images}}
        <a href="#{{.Name}}">
            <div class="thumbtext">{{.Name}}</div>
            <img src="{{.Thumb}}" />
        </a>
{{end}}
    </div>
</body>
</html>`

	files, err := ioutil.ReadDir(serveDir)
	if err != nil {
		log.Println(err)
	}

	images := []Image{}
	for _, file := range files {
		if !file.IsDir() {
			filename := path.Join(serveDir, file.Name())
			if isJPG(filename) {
				// Base filename with no extension
				baseName := strings.TrimSuffix(file.Name(), path.Ext(file.Name()))
				images = append(images, Image{Name: baseName, Thumb: path.Join("thumb", file.Name()), URL: path.Join("images", file.Name())})
			}
		}
	}

	page := ImagesPage{
		PageTitle: "Images",
		Images:    images,
	}
	tmpl, err := template.New("events").Parse(tpl)
	if err != nil {
		log.Println(err)
	}

	w.WriteHeader(http.StatusOK)
	err = tmpl.Execute(w, page)
	if err != nil {
		log.Println(err)
	}
}

func isJPG(filename string) bool {
	fileHandle, err := os.Open(filename)
	if err != nil {
		log.Println(err)
		return false
	}
	defer fileHandle.Close()

	buff := make([]byte, 512)
	if _, err = fileHandle.Read(buff); err != nil {
		log.Println(err)
		return false
	}

	if http.DetectContentType(buff) == "image/jpeg" {
		return true
	} else {
		return false
	}
}

func writeThumb(w http.ResponseWriter, r *http.Request) {
	filePath := path.Join(serveDir, path.Base(r.RequestURI))
	file, err := os.Open(filePath)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	h256 := sha256.New()
	if _, err := io.Copy(h256, file); err != nil {
		log.Println(err)
		return
	}
	h256Sum := fmt.Sprintf("%x", h256.Sum(nil))

	_, err = file.Seek(0, 0)
	if err != nil {
		log.Println(err)
		return
	}

	cacheDir := path.Join(serveDir, ".imgview")
	_, err = os.Stat(cacheDir)
	if os.IsNotExist(err) {
		_ = os.Mkdir(cacheDir, 0755)
	}
	cacheFile := path.Join(cacheDir, h256Sum)
	buffer := new(bytes.Buffer)
	hFile, err := os.Open(cacheFile)
	if err != nil {
		log.Printf("No cache for %s\n", cacheFile)

		img, err := jpeg.Decode(file)
		if err != nil {
			log.Println(err)
			return
		}

		res := resize.Thumbnail(300, 240, img, resize.Lanczos3)
		if err := jpeg.Encode(buffer, res, nil); err != nil {
			log.Println("unable to encode image.")
			return
		}

		newCache, err := os.Create(cacheFile)
		if err != nil {
			log.Println(err)
			return
		}
		defer newCache.Close()
		newCache.Write(buffer.Bytes())
	} else {
		_, err := io.Copy(buffer, hFile)
		if err != nil {
			log.Println(err)
			return
		}
	}
	defer hFile.Close()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(buffer.Bytes())))
	if _, err := w.Write(buffer.Bytes()); err != nil {
		log.Println("unable to write image.")
	}
}

func main() {
	listenString := ":8080"
	serveDir, _ = filepath.Abs(".")

	if len(os.Args) > 1 {
		listenString = os.Args[1]
	}
	if len(os.Args) > 2 {
		serveDir, _ = filepath.Abs(os.Args[2])
	}

	log.Printf("Usage: %s [address:port] [directory]", filepath.Base(os.Args[0]))
	log.Printf("Listening on: %s", listenString)
	log.Printf("Serving from: %s", serveDir)

	mux := http.NewServeMux()

	mux.HandleFunc("/status", StatusHandler)
	mux.HandleFunc("/thumb/", writeThumb)
	mux.Handle("/images/", http.StripPrefix("/images", http.FileServer(http.Dir(serveDir))))
	mux.HandleFunc("/", imageList)

	WrappedMux := logger(mux)

	log.Fatal(http.ListenAndServe(listenString, WrappedMux))
}
