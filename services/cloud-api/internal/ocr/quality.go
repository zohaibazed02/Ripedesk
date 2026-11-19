package ocr

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
)

const (
	DefaultMinBlurVariance = 120.0
	DefaultMinShortEdgePx  = 600
)

type QualityReport struct {
	Passed            bool     `json:"passed"`
	BlurScore         float64  `json:"blur_score"`
	LuminanceScore    float64  `json:"luminance_score"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	IsBlurry          bool     `json:"is_blurry"`
	Warnings          []string `json:"warnings"`
	RetakeRecommended bool     `json:"retake_recommended"`
}

// AnalyzeImageQuality runs a pure-Go grayscale Laplacian variance blur detector,
// resolution check, and luminance exposure check before paid OCR processing.
func AnalyzeImageQuality(imgBytes []byte) (*QualityReport, error) {
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 3 || h < 3 {
		return &QualityReport{
			Passed:            false,
			Width:             w,
			Height:            h,
			IsBlurry:          true,
			Warnings:          []string{"Image dimensions are too small"},
			RetakeRecommended: true,
		}, nil
	}

	// Convert to grayscale [0..255]
	gray := make([]float64, w*h)
	var lumSum float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			l := (0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8))
			gray[y*w+x] = l
			lumSum += l
		}
	}
	meanLum := lumSum / float64(w*h)

	// Discrete 3x3 Laplacian operator: [0, 1, 0; 1, -4, 1; 0, 1, 0]
	count := float64((w - 2) * (h - 2))
	var lapSum, lapSqSum float64
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			c := gray[y*w+x]
			lap := gray[(y-1)*w+x] + gray[(y+1)*w+x] + gray[y*w+(x-1)] + gray[y*w+(x+1)] - 4.0*c
			lapSum += lap
			lapSqSum += lap * lap
		}
	}
	lapMean := lapSum / count
	lapVariance := (lapSqSum / count) - (lapMean * lapMean)
	if lapVariance < 0 {
		lapVariance = 0
	}

	var warnings []string
	isBlurry := lapVariance < DefaultMinBlurVariance
	if isBlurry {
		warnings = append(warnings, fmt.Sprintf("Blurry photo detected (sharpness variance %.1f < %.1f)", lapVariance, DefaultMinBlurVariance))
	}
	shortEdge := w
	if h < shortEdge {
		shortEdge = h
	}
	if shortEdge < DefaultMinShortEdgePx {
		warnings = append(warnings, fmt.Sprintf("Low resolution (%dx%d; minimum short edge %dpx)", w, h, DefaultMinShortEdgePx))
	}
	if meanLum < 35 {
		warnings = append(warnings, "Image is underexposed / too dark")
	} else if meanLum > 242 {
		warnings = append(warnings, "Image is overexposed / washed out")
	}

	retake := isBlurry || len(warnings) > 0
	return &QualityReport{
		Passed:            !retake,
		BlurScore:         lapVariance,
		LuminanceScore:    meanLum,
		Width:             w,
		Height:            h,
		IsBlurry:          isBlurry,
		Warnings:          warnings,
		RetakeRecommended: retake,
	}, nil
}
