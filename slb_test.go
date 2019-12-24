package kslb

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRenderTemplate(t *testing.T) {
	slb := NewSLB(&SLBOpts{NgxCfg: "fixture/nginx.conf", SvcCfg: "fixture/svc.yaml"})
	slb.readConf()
	text := slb.getNginxConf() + defaultTmpl

	tmpl, err := template.New("ngxConf").Parse(text)
	if err != nil {
		logrus.Fatalf("new template error: %+v", err)
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, slb.Backend); err != nil {
		logrus.Fatalf("tmpl exec error: %+v", err)
	}

	expected := slb.getFileContent("fixture/expected.conf")
	assert.Equal(t, buf.String(), expected)
}
