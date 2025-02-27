package cli

import (
	"fmt"
	"github.com/JFryy/qq/codec"
	"github.com/JFryy/qq/internal/tui"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func CreateRootCmd() *cobra.Command {
	var inputType, outputType string
	var rawOutput bool
	var interactive bool
	var version bool
	var help bool

	cmd := &cobra.Command{
		Use:   "qq [expression] [file] [flags] \n  cat [file] | qq [expression] [flags] \n  qq -I file",
		Short: "qq - JQ processing with conversions for popular configuration formats.",
		Long:  "qq is a interoperable configuration format transcoder with jq querying ability powered by gojq. qq is multi modal, and can be used as a replacement for jq or be interacted with via a repl with autocomplete and realtime rendering preview for building queries.",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 && !cmd.Flags().Changed("input") && !cmd.Flags().Changed("output") && !cmd.Flags().Changed("raw-input") && isTerminal(os.Stdin) {
				err := cmd.Help()
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				os.Exit(0)
			}
			handleCommand(args, inputType, outputType, rawOutput, help, version, interactive)
		},
	}
	cmd.Flags().StringVarP(&inputType, "input", "i", "json", "specify input file type, only required on parsing stdin.")
	cmd.Flags().StringVarP(&outputType, "output", "o", "json", "specify output file type by extension name. This is inferred from extension if passing file position argument.")
	cmd.Flags().BoolVarP(&rawOutput, "raw-output", "r", false, "output strings without escapes and quotes.")
	cmd.Flags().BoolP("help", "h", false, "help for qq")
	cmd.Flags().BoolP("version", "v", false, "version for qq")
	cmd.Flags().BoolVarP(&interactive, "interactive", "I", false, "interactive mode for qq")

	return cmd
}

func handleCommand(args []string, inputtype string, outputtype string, rawInput bool, help bool, version bool, interactive bool) {
	var input []byte
	var err error
	var expression string
	var filename string

	if help {
		val := CreateRootCmd().Help()
		fmt.Println(val)
		os.Exit(0)
	}

	if version {
		fmt.Println("qq version 0.1.0")
		os.Exit(0)
	}
	// handle input with stdin or file
	switch len(args) {
	case 0:
		expression = "."
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case 1:
		if isFile(args[0]) {
			filename = args[0]
			expression = "."
			// read file content by name
			input, err = os.ReadFile(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

		} else {
			expression = args[0]
			input, err = io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	case 2:
		filename = args[1]
		expression = args[0]
		input, err = os.ReadFile(args[1])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	}

	var inputCodec codec.EncodingType
	if filename != "" {
		if inputtype == "json" {
			inputCodec = inferFileType(filename)
		}
	} else {
		inputCodec, err = codec.GetEncodingType(inputtype)
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	data, err := codec.Unmarshal(input, inputCodec)
	if err != nil {
		fmt.Println(err)
	}

	outputCodec, err := codec.GetEncodingType(outputtype)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if !interactive {
		query, err := gojq.Parse(expression)
		if err != nil {
			fmt.Printf("Error parsing jq expression: %v\n", err)
			os.Exit(1)
		}

		executeQuery(query, data, outputCodec, rawInput)
		os.Exit(0)
	}

	s, err := codec.Marshal(data, outputCodec)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	tui.Interact(s)
	os.Exit(0)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func inferFileType(fName string) codec.EncodingType {
	ext := strings.ToLower(filepath.Ext(fName))

	for _, t := range codec.SupportedFileTypes {
		if ext == "."+t.Ext.String() {
			return t.Ext
		}
	}
	return codec.JSON
}

func executeQuery(query *gojq.Query, data interface{}, fileType codec.EncodingType, rawOut bool) {
	iter := query.Run(data)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			fmt.Printf("Error executing jq expression: %v\n", err)
			os.Exit(1)
		}
		s, err := codec.Marshal(v, fileType)
		if err != nil {
			fmt.Printf("Error formatting result: %v\n", err)
			os.Exit(1)
		}
		r, _ := codec.PrettyFormat(s, fileType, rawOut)
		fmt.Println(r)
	}
}
