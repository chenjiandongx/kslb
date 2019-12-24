package kslb

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	defaultNgxExecPath = "/usr/sbin/nginx"
	defaultNgxConfFile = "/etc/nginx/nginx.conf"
	defaultSvcConfFile = "/etc/kslb/svc.yaml"
	defaultTmpl        = `
stream {
{{- range $index, $port := .Ports }}
	server {
		listen 0.0.0.0:{{ $port }};
		proxy_pass server_{{ $index }};
	}
{{- end }}

{{ $parent := . }}

{{- range $index, $port := .Ports }}
	upstream server_{{ $index }} {
		{{- range $parent.Servers }}
		server {{ .Host }}:{{ $port }} weight={{ .Weight }};
		{{- end }}
	}
{{- end }}
}`
)

type backendServer struct {
	Host   string `yaml:"host"`
	Weight int    `yaml:"weight"`
}

type Backend struct {
	Ports   []int           `yaml:"ports"`
	Servers []backendServer `yaml:"servers"`
}

type SLB struct {
	*Backend
	*SLBOpts
}

type SLBOpts struct {
	NgxCfg string
	SvcCfg string
}

func NewSLB(opts *SLBOpts) *SLB {
	if opts == nil {
		opts = &SLBOpts{
			NgxCfg: defaultNgxConfFile,
			SvcCfg: defaultSvcConfFile,
		}
	}
	return &SLB{SLBOpts: opts}
}

func (s *SLB) getNginxConf() string {
	c := s.getFileContent(s.NgxCfg)
	var buffer bytes.Buffer
	for _, row := range strings.Split(c, "\n") {
		if strings.HasPrefix(row, "stream") {
			break
		}
		buffer.WriteString(row + "\n")
	}
	return buffer.String()
}

func (s *SLB) getFileContent(path string) string {
	in, err := os.Open(path)
	if err != nil {
		logrus.Fatalf("open file:[%s] error: %+v", path, err)
	}

	bs, err := ioutil.ReadAll(in)
	if err != nil {
		logrus.Fatalf("read file:[%s] error: %+v", path, err)
	}
	return string(bs)
}

func (s *SLB) start() {
	process := exec.Command(defaultNgxExecPath, "-g", "daemon on;")
	if err := process.Start(); err != nil {
		logrus.Fatalf("start nginx error: %+v", err)
	}
}

func (s *SLB) reload() {
	process := exec.Command(defaultNgxExecPath, "-s", "reload")
	if err := process.Start(); err != nil {
		logrus.Fatalf("reload nginx error: %+v", err)
	}
}

func (s *SLB) watchConf() {
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Fatalf("new watcher error: %+v", err)
	}
	defer watch.Close()

	err = watch.Add(s.SvcCfg)
	if err != nil {
		logrus.Fatalf("watch file error: %+v", err)
	}

	reWatch := func() {
		s.updateConf()
		s.reload()
		if err := watch.Remove(s.SvcCfg); err != nil {
			logrus.Fatalf("watch.Remove error: %+v", err)
		}
		if err := watch.Add(s.SvcCfg); err != nil {
			logrus.Fatalf("watch.Add error: %+v", err)
		}
	}

	for {
		select {
		case ev := <-watch.Events:
			if ev.Name == s.SvcCfg {
				reWatch()
				logrus.Info("reload nginx config.")
			}
		case err := <-watch.Errors:
			logrus.Warnf("watch error: %+v", err)
		}
	}
}

func (s *SLB) readConf() {
	content := s.getFileContent(s.SvcCfg)
	bk := &Backend{}
	if err := yaml.Unmarshal([]byte(content), bk); err != nil {
		logrus.Fatalf("yaml.Unmarshal error: %+v", err)
	}
	s.Backend = bk
}

func (s *SLB) updateConf() {
	s.readConf()
	text := s.getNginxConf() + defaultTmpl

	tmpl, err := template.New("ngxConf").Parse(text)
	if err != nil {
		logrus.Fatalf("new template error: %+v", err)
	}

	f, err := os.Create(s.NgxCfg)
	if err != nil {
		logrus.Fatalf("create file [%s] error: %+v", s.NgxCfg, err)
	}

	if err = tmpl.Execute(f, s.Backend); err != nil {
		logrus.Fatalf("tmpl exec error: %+v", err)
	}
}

func (s *SLB) Run() {
	logrus.Info("Running kslb...")
	s.start()
	s.updateConf()
	go s.watchConf()

	forever := make(chan struct{})
	<-forever
}
