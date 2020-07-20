package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/nfnt/resize"
)

var serveDir string

type LoggerResponseWriter struct {
	http.ResponseWriter
	code int
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
	mux.HandleFunc("/blue", blueHandler)
	mux.HandleFunc("/red", redHandler)
	mux.HandleFunc("/thumb", writeThumb)
	mux.HandleFunc("/list", imageList)
	mux.Handle("/", http.FileServer(http.Dir(serveDir)))

	WrappedMux := logger(mux)

	log.Fatal(http.ListenAndServe(listenString, WrappedMux))
}

func imageList(w http.ResponseWriter, r *http.Request) {
	files, err := ioutil.ReadDir(serveDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if isJPG(file) {
			log.Printf("%s is a jpg", file.Name())
		}
	}
	w.WriteHeader(http.StatusOK)
}

func isJPG(file os.FileInfo) bool {
	if file.IsDir() {
		return false
	}

	fileHandle, err := os.Open(path.Join(serveDir, file.Name()))
	defer fileHandle.Close()
	if err != nil {
		log.Fatal(err)
		return false
	}

	buff := make([]byte, 512)
	if _, err = fileHandle.Read(buff); err != nil {
		log.Fatal(err)
		return false
	}

	if http.DetectContentType(buff) == "image/jpeg" {
		return true
	} else {
		return false
	}
}

func writeThumb(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open("test.jpg")
	if err != nil {
		log.Fatal(err)
	}

	img, err := jpeg.Decode(file)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()

	res := resize.Thumbnail(200, 160, img, resize.Lanczos3)
	writeImage(w, &res)
}

func blueHandler(w http.ResponseWriter, r *http.Request) {
	m := image.NewRGBA(image.Rect(0, 0, 240, 240))
	blue := color.RGBA{0, 0, 255, 255}
	draw.Draw(m, m.Bounds(), &image.Uniform{blue}, image.ZP, draw.Src)

	var img image.Image = m
	//res := resize.Thumbnail(200, 160, img, resize.Lanczos3)
	writeImage(w, &img)
}

func redHandler(w http.ResponseWriter, r *http.Request) {
	m := image.NewRGBA(image.Rect(0, 0, 240, 240))
	blue := color.RGBA{255, 0, 0, 255}
	draw.Draw(m, m.Bounds(), &image.Uniform{blue}, image.ZP, draw.Src)

	var img image.Image = m
	writeImageWithTemplate(w, &img)
}

var ImageTemplate string = `<!DOCTYPE html>
<html lang="en"><head></head>
<body><img src="data:image/jpg;base64,{{.Image}}"></body>`

// Writeimagewithtemplate encodes an image 'img' in jpeg format and writes it into ResponseWriter using a template.
func writeImageWithTemplate(w http.ResponseWriter, img *image.Image) {

	buffer := new(bytes.Buffer)
	if err := jpeg.Encode(buffer, *img, nil); err != nil {
		log.Fatalln("unable to encode image.")
	}

	str := base64.StdEncoding.EncodeToString(buffer.Bytes())
	if tmpl, err := template.New("image").Parse(ImageTemplate); err != nil {
		log.Println("unable to parse image template.")
	} else {
		data := map[string]interface{}{"Image": str}
		w.WriteHeader(http.StatusOK)
		if err = tmpl.Execute(w, data); err != nil {
			log.Println("unable to execute template.")
		}
	}
}

// writeImage encodes an image 'img' in jpeg format and writes it into ResponseWriter.
func writeImage(w http.ResponseWriter, img *image.Image) {

	buffer := new(bytes.Buffer)
	if err := jpeg.Encode(buffer, *img, nil); err != nil {
		log.Println("unable to encode image.")
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(buffer.Bytes())))
	if _, err := w.Write(buffer.Bytes()); err != nil {
		log.Println("unable to write image.")
	}
}
