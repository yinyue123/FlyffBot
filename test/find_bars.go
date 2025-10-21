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

	// 从图片看，玩家头像在左上角，血条应该在头像右边
	// 大概位置：x=45-120, y=10-30
	fmt.Println("=== Scanning for red pixels (HP bar) in player area ===")
	fmt.Println("Left upper area (around player portrait):")

	// 扫描左上角区域找红色像素
	for y := 5; y < 40; y++ {
		for x := 40; x < 130; x++ {
			b := img.GetUCharAt(y, x*3+0)
			g := img.GetUCharAt(y, x*3+1)
			r := img.GetUCharAt(y, x*3+2)

			// 检测红色：R > 150 且 R > G+30 且 R > B+30
			if r > 150 && r > g+30 && r > b+30 {
				fmt.Printf("Red pixel at (%d,%d): RGB=(%d,%d,%d)\n", x, y, r, g, b)
			}
		}
	}

	fmt.Println("\n=== Scanning for blue pixels (MP bar) in player area ===")
	// 扫描蓝色像素
	for y := 5; y < 40; y++ {
		for x := 40; x < 130; x++ {
			b := img.GetUCharAt(y, x*3+0)
			g := img.GetUCharAt(y, x*3+1)
			r := img.GetUCharAt(y, x*3+2)

			// 检测蓝色：B > 150 且 B > R+30
			if b > 150 && b > r+30 {
				fmt.Printf("Blue pixel at (%d,%d): RGB=(%d,%d,%d)\n", x, y, r, g, b)
			}
		}
	}

	fmt.Println("\n=== Scanning for green pixels (FP bar) in player area ===")
	// 扫描绿色像素
	for y := 5; y < 40; y++ {
		for x := 40; x < 130; x++ {
			b := img.GetUCharAt(y, x*3+0)
			g := img.GetUCharAt(y, x*3+1)
			r := img.GetUCharAt(y, x*3+2)

			// 检测绿色：G > 150 且 G > R+30 且 G > B+30
			if g > 150 && g > r+30 && g > b+30 {
				fmt.Printf("Green pixel at (%d,%d): RGB=(%d,%d,%d)\n", x, y, r, g, b)
			}
		}
	}

	// 扫描目标血条区域（画面中上部）
	fmt.Println("\n=== Scanning for red pixels (Target HP) in center-top area ===")
	for y := 100; y < 180; y++ {
		for x := 200; x < 500; x++ {
			b := img.GetUCharAt(y, x*3+0)
			g := img.GetUCharAt(y, x*3+1)
			r := img.GetUCharAt(y, x*3+2)

			// 检测红色
			if r > 150 && r > g+30 && r > b+30 {
				fmt.Printf("Red pixel at (%d,%d): RGB=(%d,%d,%d)\n", x, y, r, g, b)
			}
		}
	}

	fmt.Println("\nDone scanning")
}
