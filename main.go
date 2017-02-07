package main

import (
	"context"
	"encoding/json"
	"flag"
	"math/rand"

	"fmt"
	"os"

	"os/signal"
	"syscall"

	"github.com/joelanford/goscan/utils/filescanner"
	"github.com/joelanford/goscan/utils/scratch"
	"github.com/pkg/errors"
	"gopkg.in/h2non/filetype.v1"
)

type FileOpts struct {
	ScanFiles   []string
	ResultsFile string
}

var (
	rpmType = filetype.AddType("rpm", "application/x-rpm")
)

func init() {
	filetype.AddMatcher(rpmType, func(header []byte) bool {
		return len(header) >= 4 && header[0] == 0xED && header[1] == 0xAB && header[2] == 0xEE && header[3] == 0xDB
	})
}

func exit(err error, code int, ss *scratch.Scratch) {
	if ss != nil {
		if err := ss.Teardown(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(code)
}

func main() {
	//
	// Parse command line flags
	//
	scratchOpts, scanOpts, fileOpts, err := parseFlags()
	if err != nil {
		exit(err, 1, nil)
	}

	//
	// Prepare the scratch space
	//
	fmt.Printf("%+v\n", scratchOpts)
	ss := scratch.New(*scratchOpts)
	err = ss.Setup()
	if err != nil {
		exit(err, 1, nil)

	}
	defer ss.Teardown()

	//
	// Setup context and signal handlers, which will be needed
	// if we need to cleanly exit before completing the scan.
	//
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGABRT, syscall.SIGINT, syscall.SIGKILL)
	go func() {
		sig := <-sigChan
		fmt.Fprintf(os.Stderr, "Received signal %s. Exiting", sig)
		cancel()
	}()

	//
	// Setup the filescanner
	//
	fs, err := filescanner.New(*scanOpts)
	if err != nil {
		exit(err, 1, ss)
	}

	//
	// Run the scan
	//
	resChan := make(chan filescanner.ScanResult)
	err = fs.Scan(ctx, resChan, fileOpts.ScanFiles...)
	if err != nil {
		exit(err, 1, ss)
	}

	//
	// Output the hits
	//
	output, err := os.Create(fileOpts.ResultsFile)
	if err != nil {
		exit(err, 1, ss)
	}
	e := json.NewEncoder(output)
	for result := range resChan {
		err := e.Encode(result)
		if err != nil {
			exit(err, 1, ss)
		}
	}
}

func parseFlags() (*scratch.Opts, *filescanner.Opts, *FileOpts, error) {
	flag.Usage = func() {
		fmt.Printf("Usage: goscan [options] <scanfiles>\n")
		flag.PrintDefaults()
	}

	var scratchOpts scratch.Opts
	var scanOpts filescanner.Opts
	var fileOpts FileOpts

	parseScratchOpts(&scratchOpts)
	flag.StringVar(&scanOpts.KeywordsFile, "scan.words", "", "YAML keywords file")
	flag.IntVar(&scanOpts.HitContext, "scan.context", 10, "Context to capture around each hit")
	flag.StringVar(&fileOpts.ResultsFile, "output", "-", "Results output file (\"-\" for stdout)")

	flag.Parse()

	if scratchOpts.Path == "" {
		scratchOpts.Path = fmt.Sprintf("/tmp/goscan-%d", rand.Int())
	}

	scanOpts.ScratchSpacePath = scratchOpts.Path

	if fileOpts.ResultsFile == "-" {
		fileOpts.ResultsFile = "/dev/stdout"
	}

	fileOpts.ScanFiles = flag.Args()

	if scanOpts.KeywordsFile == "" {
		return nil, nil, nil, errors.New("error: scan.words file must be defined")
	}

	if len(fileOpts.ScanFiles) == 0 {
		return nil, nil, nil, errors.New("error: scan files not defined")
	}
	return &scratchOpts, &scanOpts, &fileOpts, nil
}
