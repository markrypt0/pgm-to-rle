package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
)

func parseWidthHeight(lines *[]string) (int, int) {

	// detect if this is most likely a gimp export
	if len(strings.Fields((*lines)[0])) == 1 {
		// width and height info is the 3rd gimp header line
		line := strings.Fields((*lines)[2])
		width, err := strconv.Atoi(line[0])
		if err != nil {
			log.Fatal("Unable to parse width")
		}
		height, err := strconv.Atoi(line[1])
		if err != nil {
			log.Fatal("Unable to parse height")
		}

		// drop the header info
		*lines = (*lines)[4:]
		return width, height
	}

	return len((*lines)[0]), len(*lines)
}

var screensaver = flag.Bool("screensaver", false, "generate screensaver animation")
var logo = flag.Bool("logo", false, "generate logo animation")

func main() {
	flag.Parse()

	if len(os.Args) < 2 {
		fmt.Println("Usage: pgm-to-rle [flags] file [output_struct_prefix]")
		os.Exit(1)
	}

	// read all data to buffer
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal("Unable to open image file")
	}
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal("Unable to read file")
	}

	// Name prefix for c output structs
	prefix := "REPLACE_ME"
	if len(flag.Args()) == 2 {
		prefix = flag.Arg(1)
	}

	// calculate width and height
	lines := strings.Split(string(buf), "\n")
	width, height := parseWidthHeight(&lines)

	// convert image data to flatmap
	flat := []int{}
	for _, line := range lines {

		row := strings.Fields(line)
		// ignore empty lines
		if len(row) == 0 {
			continue
		}

		// parse each value as an int and append to flattened image
		for _, v := range row {
			i, err := strconv.Atoi(v)
			if err != nil {
				log.Fatal("Data file is not formatted correctly")
			}
			flat = append(flat, i)
		}
	}

	// RLE encode the flattened image
	var result []byte
	for i := 0; i < len(flat)-1; i++ {

		var sameCount, diffCount int

		// if next digit matches it find up to (0x7f-1) matches in a row
		for ; i < len(flat)-1 && flat[i+1] == flat[i] && sameCount < (0x7f-1); i++ {
			sameCount++
		}
		// device expects the number of occurences followed by the value
		if sameCount > 0 {
			result = append(result, byte(sameCount+1))
			result = append(result, byte(flat[i]))
			continue
		}

		// else find how many different elements in a row
		diffs := []byte{}
		for ; i < len(flat)-1 && flat[i+1] != flat[i] && diffCount < 128; i++ {
			diffs = append(diffs, byte(flat[i]))
		}
		// device expects the **NEGATIVE** number of consecutive different elements up to 128, followed by each element
		if len(diffs) > 0 {
			result = append(result, byte(-1*len(diffs)))
			result = append(result, diffs...)
			i-- // decrement the counter because the current value must be the start of a new run of identical values
		}
	}

	// c-style formatting
	imgBytes := fmt.Sprintf("%#v", result)
	imgBytes = imgBytes[7 : len(imgBytes)-1] // remove go specific array formatting

	fmt.Printf("const uint8_t %s_data[%d] =\n", prefix, len(result))
	fmt.Println("{")
	fmt.Printf("    %v\n", imgBytes)
	fmt.Println("};")
	fmt.Printf("static const Image %s_image = {%d, %d, %d, %s_data};\n", prefix, width, height, len(result), prefix)

	// logo and screensaver
	if *logo {
		genLogo(result, prefix, width, height)
	}
	if *screensaver {
		genScreensaver(result, prefix, width, height)
	}
}

const (
	screenWidth  = 256
	screenHeight = 64
)

func genLogo(img []byte, prefix string, w, h int) {
	// Fade in
	fmt.Printf("\nconst VariantAnimation %s = {\n", prefix)
	fmt.Printf("    21,\n")
	fmt.Printf("    {\n")

	x := (screenWidth / 2) - (w / 2)
	y := (screenHeight / 2) - (h / 2)
	for opacity := 0; opacity <= 100; opacity += 5 {
		fmt.Printf("        {%d, %d, 25, %d, &%s_image},\n", x, y, opacity, prefix)
	}
	fmt.Printf("    }\n")
	fmt.Printf("};")

	// Fade out
	fmt.Printf("\nconst VariantAnimation %s_reversed = {\n", prefix)
	fmt.Printf("    21,\n")
	fmt.Printf("    {\n")
	for opacity := 100; opacity >= 0; opacity -= 5 {
		fmt.Printf("        {%d, %d, 25, %d, &%s_image},\n", x, y, opacity, prefix)
	}
	fmt.Printf("    }\n")
	fmt.Printf("};\n")
}

func genScreensaver(img []byte, prefix string, w, h int) {

	startX := (screenWidth / 2) - (w / 2)
	x := startX
	y := (screenHeight / 2) - (h / 2)
	// mid -> right -> left -> mid
	numFrames := (screenWidth - (x + w)) + (screenWidth - w) + startX
	fmt.Printf("\nconst VariantAnimation %s = {\n", prefix)
	fmt.Printf("    %d,\n", numFrames)
	fmt.Printf("    {\n")
	// mid->right
	for ; x+w < screenWidth-1; x++ {
		fmt.Printf("        {%d, %d, 75, 60, &%s_image},\n", x, y, prefix)
	}
	// right->left
	for ; x >= 0; x-- {
		fmt.Printf("        {%d, %d, 75, 60, &%s_image},\n", x, y, prefix)
	}
	// left->mid
	for x = 0; x <= startX; x++ {
		fmt.Printf("        {%d, %d, 75, 60, &%s_image},\n", x, y, prefix)
	}
	fmt.Printf("    }\n")
	fmt.Printf("};\n")
}
