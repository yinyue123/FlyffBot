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

	// 从图片看，玩家HP条应该在头像右边，是红色的
	// 采样具体位置
	fmt.Println("=== Player HP bar area (y=14-20, x=45-120) ===")
	for _, y := range []int{14, 15, 16, 17, 18, 19, 20} {
		hasRed := false
		minX, maxX := 999, 0
		for x := 45; x < 120; x++ {
			b := img.GetUCharAt(y, x*3+0)
			g := img.GetUCharAt(y, x*3+1)
			r := img.GetUCharAt(y, x*3+2)

			// 检测红色
			if r > 100 && r > g+10 && r > b+10 {
				hasRed = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if x == 45 || x == 60 || x == 80 || x == 100 || x == 119 {
					fmt.Printf("  (%d,%d): BGR=(%d,%d,%d)\n", x, y, b, g, r)
				}
			}
		}
		if hasRed {
			fmt.Printf("  Row %d: Red pixels from x=%d to x=%d\n", y, minX, maxX)
		}
	}

	// MP 条（蓝色）
	fmt.Println("\n=== Player MP bar area (y=22-28, x=45-120) ===")
	for _, y := range []int{22, 23, 24, 25, 26, 27, 28} {
		hasBlue := false
		minX, maxX := 999, 0
		for x := 45; x < 120; x++ {
			b := img.GetUCharAt(y, x*3+0)
			g := img.GetUCharAt(y, x*3+1)
			r := img.GetUCharAt(y, x*3+2)

			// 检测蓝色
			if b > 100 && b > r+10 {
				hasBlue = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if x == 45 || x == 60 || x == 80 || x == 100 || x == 119 {
					fmt.Printf("  (%d,%d): BGR=(%d,%d,%d)\n", x, y, b, g, r)
				}
			}
		}
		if hasBlue {
			fmt.Printf("  Row %d: Blue pixels from x=%d to x=%d\n", y, minX, maxX)
		}
	}

	// FP 条（绿色）
	fmt.Println("\n=== Player FP bar area (y=30-36, x=45-120) ===")
	for _, y := range []int{30, 31, 32, 33, 34, 35, 36} {
		hasGreen := false
		minX, maxX := 999, 0
		for x := 45; x < 120; x++ {
			b := img.GetUCharAt(y, x*3+0)
			g := img.GetUCharAt(y, x*3+1)
			r := img.GetUCharAt(y, x*3+2)

			// 检测绿色
			if g > 100 && g > r+10 && g > b+10 {
				hasGreen = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if x == 45 || x == 60 || x == 80 || x == 100 || x == 119 {
					fmt.Printf("  (%d,%d): BGR=(%d,%d,%d)\n", x, y, b, g, r)
				}
			}
		}
		if hasGreen {
			fmt.Printf("  Row %d: Green pixels from x=%d to x=%d\n", y, minX, maxX)
		}
	}

	// 目标血条
	fmt.Println("\n=== Target HP bar area (y=135-145, x=270-370) ===")
	for _, y := range []int{135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145} {
		hasRed := false
		minX, maxX := 999, 0
		for x := 270; x < 370; x++ {
			b := img.GetUCharAt(y, x*3+0)
			g := img.GetUCharAt(y, x*3+1)
			r := img.GetUCharAt(y, x*3+2)

			// 检测红色
			if r > 100 && r > g+10 && r > b+10 {
				hasRed = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if x == 270 || x == 290 || x == 310 || x == 330 || x == 350 {
					fmt.Printf("  (%d,%d): BGR=(%d,%d,%d)\n", x, y, b, g, r)
				}
			}
		}
		if hasRed {
			fmt.Printf("  Row %d: Red pixels from x=%d to x=%d\n", y, minX, maxX)
		}
	}
}
