package scratch

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"strings"

	"github.com/pkg/errors"
)

func (s *Scratch) attach() error {
	output, err := exec.Command("hdiutil", "attach", "-nomount", fmt.Sprintf("ram://%d", s.ramdiskMegabytes*2048)).CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	s.device = strings.TrimSpace(string(output))
	return nil
}

func (s *Scratch) mount() error {
	var output []byte
	var err error

	err = os.Mkdir(s.path, 0777)
	if err != nil {
		return errors.Wrapf(err, "error creating temporary directory for mount point")
	}

	output, err = exec.Command("newfs_hfs", "-v", path.Base(s.path), s.device).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "error creating hfs filesystem for ramdisk: "+string(output))
	}

	output, err = exec.Command("mount", "-o", "noatime", "-t", "hfs", s.device, s.path).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "error creating temporary directory for mount point:"+string(output))
	}

	return nil
}

func (s *Scratch) detach() error {
	output, err := exec.Command("hdiutil", "detach", s.device, "-force").CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	s.device = ""
	return nil
}
