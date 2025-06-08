//go:build integration
// +build integration

package internal

import (
	_ "embed"
	"reflect"
	"testing"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/config"
)

var (
	//go:embed test_assets/small_demo.pdf
	samplePDF []byte

	//go:embed test_assets/small_demo.pdf.jpg
	expectedJpg []byte
)

func TestConfig_RenderPageToJpg(t *testing.T) {
	type args struct {
		pdfBytes []byte
		page     int
	}
	tests := []struct {
		name    string
		c       *config.Config
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "Valid PDF with one page",
			c: &config.Config{
				Tools: config.ToolsConfig{
					ImagemagickForWindows: config.ImagemagickConfig{
						FullPath: `C:\Program Files\ImageMagick-7.1.1-Q16-HDRI\magick.exe`, // For Windows testing
					},
				},
			},
			args: args{
				pdfBytes: samplePDF,
				page:     0,
			},
			want:    expectedJpg,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderPageToJpg(tt.c, tt.args.pdfBytes, tt.args.page)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.RenderPageToJpg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// os.WriteFile(`C:\dev\paperless-ngx-privatemode-ai\test_assets\small_demo.pdf.jpg`, got, 0644)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Config.RenderPageToJpg() = %v, want %v", got, tt.want)
			}
		})
	}
}
