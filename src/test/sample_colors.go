package main

import (
	"fmt"
	"gocv.io/x/gocv"
)

func main() {
	img := gocv.IMRead("../train.png", gocv.IMReadColor)
	if img.Empty() {
		fmt.Println("Error: Could not read image")
		return
	}
	defer img.Close()

	fmt.Printf("Image size: %dx%d\n\n", img.Cols(), img.Rows())

	// Sample player HP area
	fmt.Println("=== Player HP area samples (x=105-225, y=125-135) ===")
	for _, y := range []int{125, 130, 135} {
		for _, x := range []int{105, 115, 125, 135, 145, 155} {
			if x < img.Cols() && y < img.Rows() {
				b := img.GetUCharAt(y, x*3+0)
				g := img.GetUCharAt(y, x*3+1)
				r := img.GetUCharAt(y, x*3+2)
				fmt.Printf("(%d,%d): BGR=(%d,%d,%d) RGB=(%d,%d,%d)\n", x, y, b, g, r, r, g, b)
			}
		}
	}

	// Sample player MP area
	fmt.Println("\n=== Player MP area samples (x=105-225, y=147-157) ===")
	for _, y := range []int{147, 150, 153} {
		for _, x := range []int{105, 115, 125, 135, 145, 155} {
			if x < img.Cols() && y < img.Rows() {
				b := img.GetUCharAt(y, x*3+0)
				g := img.GetUCharAt(y, x*3+1)
				r := img.GetUCharAt(y, x*3+2)
				fmt.Printf("(%d,%d): BGR=(%d,%d,%d) RGB=(%d,%d,%d)\n", x, y, b, g, r, r, g, b)
			}
		}
	}

	// Sample player FP area
	fmt.Println("\n=== Player FP area samples (x=105-225, y=169-179) ===")
	for _, y := range []int{169, 172, 175} {
		for _, x := range []int{105, 115, 125, 135, 145, 155} {
			if x < img.Cols() && y < img.Rows() {
				b := img.GetUCharAt(y, x*3+0)
				g := img.GetUCharAt(y, x*3+1)
				r := img.GetUCharAt(y, x*3+2)
				fmt.Printf("(%d,%d): BGR=(%d,%d,%d) RGB=(%d,%d,%d)\n", x, y, b, g, r, r, g, b)
			}
		}
	}

	// Sample target HP area
	fmt.Println("\n=== Target HP area samples (x=300-550, y=35-45) ===")
	for _, y := range []int{35, 40, 45} {
		for _, x := range []int{300, 350, 400, 450, 500, 550} {
			if x < img.Cols() && y < img.Rows() {
				b := img.GetUCharAt(y, x*3+0)
				g := img.GetUCharAt(y, x*3+1)
				r := img.GetUCharAt(y, x*3+2)
				fmt.Printf("(%d,%d): BGR=(%d,%d,%d) RGB=(%d,%d,%d)\n", x, y, b, g, r, r, g, b)
			}
		}
	}
}
