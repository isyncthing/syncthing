// Copyright (C) 2014 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package config

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/d4l3k/messagediff"

	"github.com/syncthing/syncthing/lib/events"
	"github.com/syncthing/syncthing/lib/fs"
	"github.com/syncthing/syncthing/lib/protocol"
)

var device1, device2, device3, device4 protocol.DeviceID

func init() {
	device1, _ = protocol.DeviceIDFromString("AIR6LPZ7K4PTTUXQSMUUCPQ5YWOEDFIIQJUG7772YQXXR5YD6AWQ")
	device2, _ = protocol.DeviceIDFromString("GYRZZQB-IRNPV4Z-T7TC52W-EQYJ3TT-FDQW6MW-DFLMU42-SSSU6EM-FBK2VAY")
	device3, _ = protocol.DeviceIDFromString("LGFPDIT-7SKNNJL-VJZA4FC-7QNCRKA-CE753K7-2BW5QDK-2FOZ7FR-FEP57QJ")
	device4, _ = protocol.DeviceIDFromString("P56IOI7-MZJNU2Y-IQGDREY-DM2MGTI-MGL3BXN-PQ6W5BM-TBBZ4TJ-XZWICQ2")
}

func TestDefaultValues(t *testing.T) {
	expected := OptionsConfiguration{
		RawListenAddresses:      []string{"default"},
		RawGlobalAnnServers:     []string{"default"},
		GlobalAnnEnabled:        true,
		LocalAnnEnabled:         true,
		LocalAnnPort:            21027,
		LocalAnnMCAddr:          "[ff12::8384]:21027",
		MaxSendKbps:             0,
		MaxRecvKbps:             0,
		ReconnectIntervalS:      60,
		RelaysEnabled:           true,
		RelayReconnectIntervalM: 10,
		StartBrowser:            true,
		NATEnabled:              true,
		NATLeaseM:               60,
		NATRenewalM:             30,
		NATTimeoutS:             10,
		RestartOnWakeup:         true,
		AutoUpgradeIntervalH:    12,
		KeepTemporariesH:        24,
		CacheIgnoredFiles:       false,
		ProgressUpdateIntervalS: 5,
		LimitBandwidthInLan:     false,
		MinHomeDiskFree:         Size{1, "%"},
		URURL:                   "https://data.syncthing.net/newdata",
		URInitialDelayS:         1800,
		URPostInsecurely:        false,
		ReleasesURL:             "https://upgrades.syncthing.net/meta.json",
		AlwaysLocalNets:         []string{},
		OverwriteRemoteDevNames: false,
		TempIndexMinBlocks:      10,
		UnackedNotificationIDs:  []string{"authenticationUserAndPassword"},
		DefaultFolderPath:       "~",
		SetLowPriority:          true,
		CRURL:                   "https://crash.syncthing.net/newcrash",
		CREnabled:               true,
		StunKeepaliveStartS:     180,
		StunKeepaliveMinS:       20,
		RawStunServers:          []string{"default"},
		AnnounceLANAddresses:    true,
	}

	cfg := New(device1)

	if diff, equal := messagediff.PrettyDiff(expected, cfg.Options); !equal {
		t.Errorf("Default config differs. Diff:\n%s", diff)
	}
}

func TestDeviceConfig(t *testing.T) {
	for i := OldestHandledVersion; i <= CurrentVersion; i++ {
		cfgFile := fmt.Sprintf("testdata/v%d.xml", i)
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			continue
		}

		os.RemoveAll(filepath.Join("testdata", DefaultMarkerName))
		wr, err := load(cfgFile, device1)
		if err != nil {
			t.Fatal(err)
		}

		_, err = os.Stat(filepath.Join("testdata", DefaultMarkerName))
		if i < 6 && err != nil {
			t.Fatal(err)
		} else if i >= 6 && err == nil {
			t.Fatal("Unexpected file")
		}

		cfg := wr.(*wrapper).cfg

		expectedFolders := []FolderConfiguration{
			{
				ID:               "test",
				FilesystemType:   fs.FilesystemTypeBasic,
				Path:             "testdata",
				Devices:          []FolderDeviceConfiguration{{DeviceID: device1}, {DeviceID: device4}},
				Type:             FolderTypeSendOnly,
				RescanIntervalS:  600,
				FSWatcherEnabled: false,
				FSWatcherDelayS:  10,
				Copiers:          0,
				Hashers:          0,
				AutoNormalize:    true,
				MinDiskFree:      Size{1, "%"},
				MaxConflicts:     -1,
				Versioning: VersioningConfiguration{
					Params: map[string]string{},
				},
				WeakHashThresholdPct: 25,
				MarkerName:           DefaultMarkerName,
				JunctionsAsDirs:      true,
				MaxConcurrentWrites:  maxConcurrentWritesDefault,
			},
		}

		expectedDevices := []DeviceConfiguration{
			{
				DeviceID:        device1,
				Name:            "node one",
				Addresses:       []string{"tcp://a"},
				Compression:     protocol.CompressionMetadata,
				AllowedNetworks: []string{},
				IgnoredFolders:  []ObservedFolder{},
				PendingFolders:  []ObservedFolder{},
			},
			{
				DeviceID:        device4,
				Name:            "node two",
				Addresses:       []string{"tcp://b"},
				Compression:     protocol.CompressionMetadata,
				AllowedNetworks: []string{},
				IgnoredFolders:  []ObservedFolder{},
				PendingFolders:  []ObservedFolder{},
			},
		}
		expectedDeviceIDs := []protocol.DeviceID{device1, device4}

		if cfg.Version != CurrentVersion {
			t.Errorf("%d: Incorrect version %d != %d", i, cfg.Version, CurrentVersion)
		}
		if diff, equal := messagediff.PrettyDiff(expectedFolders, cfg.Folders); !equal {
			t.Errorf("%d: Incorrect Folders. Diff:\n%s", i, diff)
		}
		if diff, equal := messagediff.PrettyDiff(expectedDevices, cfg.Devices); !equal {
			t.Errorf("%d: Incorrect Devices. Diff:\n%s", i, diff)
		}
		if diff, equal := messagediff.PrettyDiff(expectedDeviceIDs, cfg.Folders[0].DeviceIDs()); !equal {
			t.Errorf("%d: Incorrect DeviceIDs. Diff:\n%s", i, diff)
		}
	}
}

func TestNoListenAddresses(t *testing.T) {
	cfg, err := load("testdata/nolistenaddress.xml", device1)
	if err != nil {
		t.Error(err)
	}

	expected := []string{""}
	actual := cfg.Options().RawListenAddresses
	if diff, equal := messagediff.PrettyDiff(expected, actual); !equal {
		t.Errorf("Unexpected RawListenAddresses. Diff:\n%s", diff)
	}
}

func TestOverriddenValues(t *testing.T) {
	expected := OptionsConfiguration{
		RawListenAddresses:      []string{"tcp://:23000"},
		RawGlobalAnnServers:     []string{"udp4://syncthing.nym.se:22026"},
		GlobalAnnEnabled:        false,
		LocalAnnEnabled:         false,
		LocalAnnPort:            42123,
		LocalAnnMCAddr:          "quux:3232",
		MaxSendKbps:             1234,
		MaxRecvKbps:             2341,
		ReconnectIntervalS:      6000,
		RelaysEnabled:           false,
		RelayReconnectIntervalM: 20,
		StartBrowser:            false,
		NATEnabled:              false,
		NATLeaseM:               90,
		NATRenewalM:             15,
		NATTimeoutS:             15,
		RestartOnWakeup:         false,
		AutoUpgradeIntervalH:    24,
		KeepTemporariesH:        48,
		CacheIgnoredFiles:       true,
		ProgressUpdateIntervalS: 10,
		LimitBandwidthInLan:     true,
		MinHomeDiskFree:         Size{5.2, "%"},
		URSeen:                  8,
		URAccepted:              4,
		URURL:                   "https://localhost/newdata",
		URInitialDelayS:         800,
		URPostInsecurely:        true,
		ReleasesURL:             "https://localhost/releases",
		AlwaysLocalNets:         []string{},
		OverwriteRemoteDevNames: true,
		TempIndexMinBlocks:      100,
		UnackedNotificationIDs:  []string{"asdfasdf"},
		DefaultFolderPath:       "/media/syncthing",
		SetLowPriority:          false,
		CRURL:                   "https://localhost/newcrash",
		CREnabled:               false,
		StunKeepaliveStartS:     9000,
		StunKeepaliveMinS:       900,
		RawStunServers:          []string{"foo"},
	}

	os.Unsetenv("STNOUPGRADE")
	cfg, err := load("testdata/overridenvalues.xml", device1)
	if err != nil {
		t.Error(err)
	}

	if diff, equal := messagediff.PrettyDiff(expected, cfg.Options()); !equal {
		t.Errorf("Overridden config differs. Diff:\n%s", diff)
	}
}

func TestDeviceAddressesDynamic(t *testing.T) {
	name, _ := os.Hostname()
	expected := map[protocol.DeviceID]DeviceConfiguration{
		device1: {
			DeviceID:        device1,
			Addresses:       []string{"dynamic"},
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
		device2: {
			DeviceID:        device2,
			Addresses:       []string{"dynamic"},
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
		device3: {
			DeviceID:        device3,
			Addresses:       []string{"dynamic"},
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
		device4: {
			DeviceID:        device4,
			Name:            name, // Set when auto created
			Addresses:       []string{"dynamic"},
			Compression:     protocol.CompressionMetadata,
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
	}

	cfg, err := load("testdata/deviceaddressesdynamic.xml", device4)
	if err != nil {
		t.Error(err)
	}

	actual := cfg.Devices()
	if diff, equal := messagediff.PrettyDiff(expected, actual); !equal {
		t.Errorf("Devices differ. Diff:\n%s", diff)
	}
}

func TestDeviceCompression(t *testing.T) {
	name, _ := os.Hostname()
	expected := map[protocol.DeviceID]DeviceConfiguration{
		device1: {
			DeviceID:        device1,
			Addresses:       []string{"dynamic"},
			Compression:     protocol.CompressionMetadata,
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
		device2: {
			DeviceID:        device2,
			Addresses:       []string{"dynamic"},
			Compression:     protocol.CompressionMetadata,
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
		device3: {
			DeviceID:        device3,
			Addresses:       []string{"dynamic"},
			Compression:     protocol.CompressionNever,
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
		device4: {
			DeviceID:        device4,
			Name:            name, // Set when auto created
			Addresses:       []string{"dynamic"},
			Compression:     protocol.CompressionMetadata,
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
	}

	cfg, err := load("testdata/devicecompression.xml", device4)
	if err != nil {
		t.Error(err)
	}

	actual := cfg.Devices()
	if diff, equal := messagediff.PrettyDiff(expected, actual); !equal {
		t.Errorf("Devices differ. Diff:\n%s", diff)
	}
}

func TestDeviceAddressesStatic(t *testing.T) {
	name, _ := os.Hostname()
	expected := map[protocol.DeviceID]DeviceConfiguration{
		device1: {
			DeviceID:        device1,
			Addresses:       []string{"tcp://192.0.2.1", "tcp://192.0.2.2"},
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
		device2: {
			DeviceID:        device2,
			Addresses:       []string{"tcp://192.0.2.3:6070", "tcp://[2001:db8::42]:4242"},
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
		device3: {
			DeviceID:        device3,
			Addresses:       []string{"tcp://[2001:db8::44]:4444", "tcp://192.0.2.4:6090"},
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
		device4: {
			DeviceID:        device4,
			Name:            name, // Set when auto created
			Addresses:       []string{"dynamic"},
			Compression:     protocol.CompressionMetadata,
			AllowedNetworks: []string{},
			IgnoredFolders:  []ObservedFolder{},
			PendingFolders:  []ObservedFolder{},
		},
	}

	cfg, err := load("testdata/deviceaddressesstatic.xml", device4)
	if err != nil {
		t.Error(err)
	}

	actual := cfg.Devices()
	if diff, equal := messagediff.PrettyDiff(expected, actual); !equal {
		t.Errorf("Devices differ. Diff:\n%s", diff)
	}
}

func TestVersioningConfig(t *testing.T) {
	cfg, err := load("testdata/versioningconfig.xml", device4)
	if err != nil {
		t.Error(err)
	}

	vc := cfg.Folders()["test"].Versioning
	if vc.Type != "simple" {
		t.Errorf(`vc.Type %q != "simple"`, vc.Type)
	}
	if l := len(vc.Params); l != 2 {
		t.Errorf("len(vc.Params) %d != 2", l)
	}

	expected := map[string]string{
		"foo": "bar",
		"baz": "quux",
	}
	if diff, equal := messagediff.PrettyDiff(expected, vc.Params); !equal {
		t.Errorf("vc.Params differ. Diff:\n%s", diff)
	}
}

func TestIssue1262(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("path gets converted to absolute as part of the filesystem initialization on linux")
	}

	cfg, err := load("testdata/issue-1262.xml", device4)
	if err != nil {
		t.Fatal(err)
	}

	actual := cfg.Folders()["test"].Filesystem().URI()
	expected := `e:\`

	if actual != expected {
		t.Errorf("%q != %q", actual, expected)
	}
}

func TestIssue1750(t *testing.T) {
	cfg, err := load("testdata/issue-1750.xml", device4)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Options().RawListenAddresses[0] != "tcp://:23000" {
		t.Errorf("%q != %q", cfg.Options().RawListenAddresses[0], "tcp://:23000")
	}

	if cfg.Options().RawListenAddresses[1] != "tcp://:23001" {
		t.Errorf("%q != %q", cfg.Options().RawListenAddresses[1], "tcp://:23001")
	}

	if cfg.Options().RawGlobalAnnServers[0] != "udp4://syncthing.nym.se:22026" {
		t.Errorf("%q != %q", cfg.Options().RawGlobalAnnServers[0], "udp4://syncthing.nym.se:22026")
	}

	if cfg.Options().RawGlobalAnnServers[1] != "udp4://syncthing.nym.se:22027" {
		t.Errorf("%q != %q", cfg.Options().RawGlobalAnnServers[1], "udp4://syncthing.nym.se:22027")
	}
}

func TestFolderPath(t *testing.T) {
	folder := FolderConfiguration{
		Path: "~/tmp",
	}

	realPath := folder.Filesystem().URI()
	if !filepath.IsAbs(realPath) {
		t.Error(realPath, "should be absolute")
	}
	if strings.Contains(realPath, "~") {
		t.Error(realPath, "should not contain ~")
	}
}

func TestFolderCheckPath(t *testing.T) {
	n, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	testFs := fs.NewFilesystem(fs.FilesystemTypeBasic, n)

	err = os.MkdirAll(filepath.Join(n, "dir", ".stfolder"), os.FileMode(0777))
	if err != nil {
		t.Fatal(err)
	}

	testcases := []struct {
		path string
		err  error
	}{
		{
			path: "",
			err:  ErrMarkerMissing,
		},
		{
			path: "does not exist",
			err:  ErrPathMissing,
		},
		{
			path: "dir",
			err:  nil,
		},
	}

	err = fs.DebugSymlinkForTestsOnly(testFs, testFs, "dir", "link")
	if err == nil {
		t.Log("running with symlink check")
		testcases = append(testcases, struct {
			path string
			err  error
		}{
			path: "link",
			err:  nil,
		})
	} else if runtime.GOOS != "windows" {
		t.Log("running without symlink check")
		t.Fatal(err)
	}

	for _, testcase := range testcases {
		cfg := FolderConfiguration{
			Path:       filepath.Join(n, testcase.path),
			MarkerName: DefaultMarkerName,
		}

		if err := cfg.CheckPath(); testcase.err != err {
			t.Errorf("unexpected error in case %s: %s != %s", testcase.path, err, testcase.err)
		}
	}
}

func TestNewSaveLoad(t *testing.T) {
	path := "testdata/temp.xml"
	os.Remove(path)

	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	intCfg := New(device1)
	cfg := wrap(path, intCfg)

	if exists(path) {
		t.Error(path, "exists")
	}

	err := cfg.Save()
	if err != nil {
		t.Error(err)
	}
	if !exists(path) {
		t.Error(path, "does not exist")
	}

	cfg2, err := load(path, device1)
	if err != nil {
		t.Error(err)
	}

	if diff, equal := messagediff.PrettyDiff(cfg.RawCopy(), cfg2.RawCopy()); !equal {
		t.Errorf("Configs are not equal. Diff:\n%s", diff)
	}

	os.Remove(path)
}

func TestPrepare(t *testing.T) {
	var cfg Configuration

	if cfg.Folders != nil || cfg.Devices != nil || cfg.Options.RawListenAddresses != nil {
		t.Error("Expected nil")
	}

	cfg.prepare(device1)

	if cfg.Folders == nil || cfg.Devices == nil || cfg.Options.RawListenAddresses == nil {
		t.Error("Unexpected nil")
	}
}

func TestCopy(t *testing.T) {
	wrapper, err := load("testdata/example.xml", device1)
	if err != nil {
		t.Fatal(err)
	}
	cfg := wrapper.RawCopy()

	bsOrig, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	copy := cfg.Copy()

	cfg.Devices[0].Addresses[0] = "wrong"
	cfg.Folders[0].Devices[0].DeviceID = protocol.DeviceID{0, 1, 2, 3}
	cfg.Options.RawListenAddresses[0] = "wrong"
	cfg.GUI.APIKey = "wrong"

	bsChanged, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	bsCopy, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(bsOrig, bsChanged) {
		t.Error("Config should have changed")
	}
	if !bytes.Equal(bsOrig, bsCopy) {
		// ioutil.WriteFile("a", bsOrig, 0644)
		// ioutil.WriteFile("b", bsCopy, 0644)
		t.Error("Copy should be unchanged")
	}
}

func TestPullOrder(t *testing.T) {
	wrapper, err := load("testdata/pullorder.xml", device1)
	if err != nil {
		t.Fatal(err)
	}
	folders := wrapper.Folders()

	expected := []struct {
		name  string
		order PullOrder
	}{
		{"f1", PullOrderRandom},        // empty value, default
		{"f2", PullOrderRandom},        // explicit
		{"f3", PullOrderAlphabetic},    // explicit
		{"f4", PullOrderRandom},        // unknown value, default
		{"f5", PullOrderSmallestFirst}, // explicit
		{"f6", PullOrderLargestFirst},  // explicit
		{"f7", PullOrderOldestFirst},   // explicit
		{"f8", PullOrderNewestFirst},   // explicit
	}

	// Verify values are deserialized correctly

	for _, tc := range expected {
		if actual := folders[tc.name].Order; actual != tc.order {
			t.Errorf("Incorrect pull order for %q: %v != %v", tc.name, actual, tc.order)
		}
	}

	// Serialize and deserialize again to verify it survives the transformation

	buf := new(bytes.Buffer)
	cfg := wrapper.RawCopy()
	cfg.WriteXML(buf)

	t.Logf("%s", buf.Bytes())

	cfg, _, err = ReadXML(buf, device1)
	if err != nil {
		t.Fatal(err)
	}
	wrapper = wrap("testdata/pullorder.xml", cfg)
	folders = wrapper.Folders()

	for _, tc := range expected {
		if actual := folders[tc.name].Order; actual != tc.order {
			t.Errorf("Incorrect pull order for %q: %v != %v", tc.name, actual, tc.order)
		}
	}
}

func TestLargeRescanInterval(t *testing.T) {
	wrapper, err := load("testdata/largeinterval.xml", device1)
	if err != nil {
		t.Fatal(err)
	}

	if wrapper.Folders()["l1"].RescanIntervalS != MaxRescanIntervalS {
		t.Error("too large rescan interval should be maxed out")
	}
	if wrapper.Folders()["l2"].RescanIntervalS != 0 {
		t.Error("negative rescan interval should become zero")
	}
}

func TestGUIConfigURL(t *testing.T) {
	testcases := [][2]string{
		{"192.0.2.42:8080", "http://192.0.2.42:8080/"},
		{":8080", "http://127.0.0.1:8080/"},
		{"0.0.0.0:8080", "http://127.0.0.1:8080/"},
		{"127.0.0.1:8080", "http://127.0.0.1:8080/"},
		{"127.0.0.2:8080", "http://127.0.0.2:8080/"},
		{"[::]:8080", "http://[::1]:8080/"},
		{"[2001::42]:8080", "http://[2001::42]:8080/"},
	}

	for _, tc := range testcases {
		c := GUIConfiguration{
			RawAddress: tc[0],
		}
		u := c.URL()
		if u != tc[1] {
			t.Errorf("Incorrect URL %s != %s for addr %s", u, tc[1], tc[0])
		}
	}
}

func TestDuplicateDevices(t *testing.T) {
	// Duplicate devices should be removed

	wrapper, err := load("testdata/dupdevices.xml", device1)
	if err != nil {
		t.Fatal(err)
	}

	if l := len(wrapper.RawCopy().Devices); l != 3 {
		t.Errorf("Incorrect number of devices, %d != 3", l)
	}

	f := wrapper.Folders()["f2"]
	if l := len(f.Devices); l != 2 {
		t.Errorf("Incorrect number of folder devices, %d != 2", l)
	}
}

func TestDuplicateFolders(t *testing.T) {
	// Duplicate folders are a loading error

	_, err := load("testdata/dupfolders.xml", device1)
	if err == nil || !strings.Contains(err.Error(), errFolderIDDuplicate.Error()) {
		t.Fatal(`Expected error to mention "duplicate folder ID":`, err)
	}
}

func TestEmptyFolderPaths(t *testing.T) {
	// Empty folder paths are not allowed at the loading stage, and should not
	// get messed up by the prepare steps (e.g., become the current dir or
	// get a slash added so that it becomes the root directory or similar).

	_, err := load("testdata/nopath.xml", device1)
	if err == nil || !strings.Contains(err.Error(), errFolderPathEmpty.Error()) {
		t.Fatal("Expected error due to empty folder path, got", err)
	}
}

func TestV14ListenAddressesMigration(t *testing.T) {
	tcs := [][3][]string{
		// Default listen plus default relays is now "default"
		{
			{"tcp://0.0.0.0:22000"},
			{"dynamic+https://relays.syncthing.net/endpoint"},
			{"default"},
		},
		// Default listen address without any relay addresses gets converted
		// to just the listen address. It's easier this way, and frankly the
		// user has gone to some trouble to get the empty string in the
		// config to start with...
		{
			{"tcp://0.0.0.0:22000"}, // old listen addrs
			{""},                    // old relay addrs
			{"tcp://0.0.0.0:22000"}, // new listen addrs
		},
		// Default listen plus non-default relays gets copied verbatim
		{
			{"tcp://0.0.0.0:22000"},
			{"dynamic+https://other.example.com"},
			{"tcp://0.0.0.0:22000", "dynamic+https://other.example.com"},
		},
		// Non-default listen plus default relays gets copied verbatim
		{
			{"tcp://1.2.3.4:22000"},
			{"dynamic+https://relays.syncthing.net/endpoint"},
			{"tcp://1.2.3.4:22000", "dynamic+https://relays.syncthing.net/endpoint"},
		},
		// Default stuff gets sucked into "default", the rest gets copied
		{
			{"tcp://0.0.0.0:22000", "tcp://1.2.3.4:22000"},
			{"dynamic+https://relays.syncthing.net/endpoint", "relay://other.example.com"},
			{"default", "tcp://1.2.3.4:22000", "relay://other.example.com"},
		},
	}

	m := migration{14, migrateToConfigV14}

	for _, tc := range tcs {
		cfg := Configuration{
			Version: 13,
			Options: OptionsConfiguration{
				RawListenAddresses:     tc[0],
				DeprecatedRelayServers: tc[1],
			},
		}
		m.apply(&cfg)
		if cfg.Version != 14 {
			t.Error("Configuration was not converted")
		}

		sort.Strings(tc[2])
		if !reflect.DeepEqual(cfg.Options.RawListenAddresses, tc[2]) {
			t.Errorf("Migration error; actual %#v != expected %#v", cfg.Options.RawListenAddresses, tc[2])
		}
	}
}

func TestIgnoredDevices(t *testing.T) {
	// Verify that ignored devices that are also present in the
	// configuration are not in fact ignored.

	wrapper, err := load("testdata/ignoreddevices.xml", device1)
	if err != nil {
		t.Fatal(err)
	}

	if wrapper.IgnoredDevice(device1) {
		t.Errorf("Device %v should not be ignored", device1)
	}
	if !wrapper.IgnoredDevice(device3) {
		t.Errorf("Device %v should be ignored", device3)
	}
}

func TestIgnoredFolders(t *testing.T) {
	// Verify that ignored folder that are also present in the
	// configuration are not in fact ignored.
	// Also, verify that folders that are shared with a device are not ignored.

	wrapper, err := load("testdata/ignoredfolders.xml", device1)
	if err != nil {
		t.Fatal(err)
	}

	if wrapper.IgnoredFolder(device2, "folder1") {
		t.Errorf("Device %v should not be ignored", device2)
	}
	if !wrapper.IgnoredFolder(device3, "folder1") {
		t.Errorf("Device %v should be ignored", device3)
	}
	// Should be removed, hence not ignored.
	if wrapper.IgnoredFolder(device4, "folder1") {
		t.Errorf("Device %v should not be ignored", device4)
	}

	if !wrapper.IgnoredFolder(device2, "folder2") {
		t.Errorf("Device %v should not be ignored", device2)
	}
	if !wrapper.IgnoredFolder(device3, "folder2") {
		t.Errorf("Device %v should be ignored", device3)
	}

	// 2 for folder2, 1 for folder1, as non-existing device and device the folder is shared with is removed.
	expectedIgnoredFolders := 3
	for _, dev := range wrapper.Devices() {
		expectedIgnoredFolders -= len(dev.IgnoredFolders)
	}
	if expectedIgnoredFolders != 0 {
		t.Errorf("Left with %d ignored folders", expectedIgnoredFolders)
	}
}

func TestGetDevice(t *testing.T) {
	// Verify that the Device() call does the right thing

	wrapper, err := load("testdata/ignoreddevices.xml", device1)
	if err != nil {
		t.Fatal(err)
	}

	// device1 is mentioned in the config

	device, ok := wrapper.Device(device1)
	if !ok {
		t.Error(device1, "should exist")
	}
	if device.DeviceID != device1 {
		t.Error("Should have returned", device1, "not", device.DeviceID)
	}

	// device3 is not

	device, ok = wrapper.Device(device3)
	if ok {
		t.Error(device3, "should not exist")
	}
	if device.DeviceID == device3 {
		t.Error("Should not returned ID", device3)
	}
}

func TestSharesRemovedOnDeviceRemoval(t *testing.T) {
	wrapper, err := load("testdata/example.xml", device1)
	if err != nil {
		t.Errorf("Failed: %s", err)
	}

	raw := wrapper.RawCopy()
	raw.Devices = raw.Devices[:len(raw.Devices)-1]

	if len(raw.Folders[0].Devices) <= len(raw.Devices) {
		t.Error("Should have less devices")
	}

	_, err = wrapper.Replace(raw)
	if err != nil {
		t.Errorf("Failed: %s", err)
	}

	raw = wrapper.RawCopy()
	if len(raw.Folders[0].Devices) > len(raw.Devices) {
		t.Error("Unexpected extra device")
	}
}

func TestIssue4219(t *testing.T) {
	// Adding a folder that was previously ignored should make it unignored.

	r := bytes.NewReader([]byte(`{
		"devices": [
			{
				"deviceID": "GYRZZQB-IRNPV4Z-T7TC52W-EQYJ3TT-FDQW6MW-DFLMU42-SSSU6EM-FBK2VAY",
				"ignoredFolders": [
					{
						"id": "t1"
					},
					{
						"id": "abcd123"
					}
				]
			},
			{
				"deviceID": "LGFPDIT-7SKNNJL-VJZA4FC-7QNCRKA-CE753K7-2BW5QDK-2FOZ7FR-FEP57QJ",
				"ignoredFolders": [
					{
						"id": "t1"
					},
					{
						"id": "abcd123"
					}
				]
			}
		],
		"folders": [
			{
				"id": "abcd123",
				"path": "testdata",
				"devices":[
					{"deviceID": "GYRZZQB-IRNPV4Z-T7TC52W-EQYJ3TT-FDQW6MW-DFLMU42-SSSU6EM-FBK2VAY"}
				]
			}
		],
		"remoteIgnoredDevices": [
			{
				"deviceID": "GYRZZQB-IRNPV4Z-T7TC52W-EQYJ3TT-FDQW6MW-DFLMU42-SSSU6EM-FBK2VAY"
			}
		]
	}`))

	cfg, err := ReadJSON(r, protocol.LocalDeviceID)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.IgnoredDevices) != 0 { // 1 gets removed
		t.Errorf("There should be zero ignored devices, not %d", len(cfg.IgnoredDevices))
	}

	ignoredFolders := 0
	for _, dev := range cfg.Devices {
		ignoredFolders += len(dev.IgnoredFolders)
	}

	if ignoredFolders != 3 { // 1 gets removed
		t.Errorf("There should be three ignored folders, not %d", ignoredFolders)
	}

	w := wrap("/tmp/cfg", cfg)
	if !w.IgnoredFolder(device2, "t1") {
		t.Error("Folder device2 t1 should be ignored")
	}
	if !w.IgnoredFolder(device3, "t1") {
		t.Error("Folder device3 t1 should be ignored")
	}
	if w.IgnoredFolder(device2, "abcd123") {
		t.Error("Folder device2 abcd123 should not be ignored")
	}
	if !w.IgnoredFolder(device3, "abcd123") {
		t.Error("Folder device3 abcd123 should be ignored")
	}
}

func TestInvalidDeviceIDRejected(t *testing.T) {
	// This test verifies that we properly reject invalid device IDs when
	// deserializing a JSON config.

	cases := []struct {
		id string
		ok bool
	}{
		// a genuine device ID
		{"GYRZZQB-IRNPV4Z-T7TC52W-EQYJ3TT-FDQW6MW-DFLMU42-SSSU6EM-FBK2VAY", true},
		// incorrect check digit
		{"GYRZZQB-IRNPV4A-T7TC52W-EQYJ3TT-FDQW6MW-DFLMU42-SSSU6EM-FBK2VAY", false},
		// missing digit
		{"GYRZZQB-IRNPV4Z-T7TC52W-EQYJ3TT-FDQW6MW-DFLMU42-SSSU6EM-FBK2VA", false},
		// clearly broken
		{"invalid", false},
		// accepted as the empty device ID for historical reasons...
		{"", true},
	}

	for _, tc := range cases {
		cfg := defaultConfigAsMap()

		// Change the device ID of the first device to "invalid". Fast and loose
		// with the type assertions as we know what the JSON decoder returns.
		devs := cfg["devices"].([]interface{})
		dev0 := devs[0].(map[string]interface{})
		dev0["deviceID"] = tc.id
		devs[0] = dev0

		invalidJSON, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}

		_, err = ReadJSON(bytes.NewReader(invalidJSON), device1)
		if tc.ok && err != nil {
			t.Errorf("unexpected error for device ID %q: %v", tc.id, err)
		} else if !tc.ok && err == nil {
			t.Errorf("device ID %q, expected error but got nil", tc.id)
		}
	}
}

func TestInvalidFolderIDRejected(t *testing.T) {
	// This test verifies that we properly reject invalid folder IDs when
	// deserializing a JSON config.

	cases := []struct {
		id string
		ok bool
	}{
		// a reasonable folder ID
		{"foo", true},
		// empty is not OK
		{"", false},
	}

	for _, tc := range cases {
		cfg := defaultConfigAsMap()

		// Change the folder ID of the first folder to the empty string.
		// Fast and loose with the type assertions as we know what the JSON
		// decoder returns.
		devs := cfg["folders"].([]interface{})
		dev0 := devs[0].(map[string]interface{})
		dev0["id"] = tc.id
		devs[0] = dev0

		invalidJSON, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}

		_, err = ReadJSON(bytes.NewReader(invalidJSON), device1)
		if tc.ok && err != nil {
			t.Errorf("unexpected error for folder ID %q: %v", tc.id, err)
		} else if !tc.ok && err == nil {
			t.Errorf("folder ID %q, expected error but got nil", tc.id)
		}
	}
}

func TestFilterURLSchemePrefix(t *testing.T) {
	cases := []struct {
		before []string
		prefix string
		after  []string
	}{
		{[]string{}, "kcp", []string{}},
		{[]string{"tcp://foo"}, "kcp", []string{"tcp://foo"}},
		{[]string{"kcp://foo"}, "kcp", []string{}},
		{[]string{"tcp6://foo", "kcp6://foo"}, "kcp", []string{"tcp6://foo"}},
		{[]string{"kcp6://foo", "tcp6://foo"}, "kcp", []string{"tcp6://foo"}},
		{
			[]string{"tcp://foo", "tcp4://foo", "kcp://foo", "kcp4://foo", "banana://foo", "banana4://foo", "banananas!"},
			"kcp",
			[]string{"tcp://foo", "tcp4://foo", "banana://foo", "banana4://foo", "banananas!"},
		},
	}

	for _, tc := range cases {
		res := filterURLSchemePrefix(tc.before, tc.prefix)
		if !reflect.DeepEqual(res, tc.after) {
			t.Errorf("filterURLSchemePrefix => %q, expected %q", res, tc.after)
		}
	}
}

func TestDeviceConfigObservedNotNil(t *testing.T) {
	cfg := Configuration{
		Devices: []DeviceConfiguration{
			{},
		},
	}

	cfg.prepare(device1)

	for _, dev := range cfg.Devices {
		if dev.IgnoredFolders == nil {
			t.Errorf("Ignored folders nil")
		}

		if dev.PendingFolders == nil {
			t.Errorf("Pending folders nil")
		}
	}
}

func TestRemoveDeviceWithEmptyID(t *testing.T) {
	cfg := Configuration{
		Devices: []DeviceConfiguration{
			{
				Name: "foo",
			},
		},
		Folders: []FolderConfiguration{
			{
				ID:      "foo",
				Path:    "testdata",
				Devices: []FolderDeviceConfiguration{{}},
			},
		},
	}

	cfg.clean()

	if len(cfg.Devices) != 0 {
		t.Error("Expected device with empty ID to be removed from config:", cfg.Devices)
	}
	if len(cfg.Folders[0].Devices) != 0 {
		t.Error("Expected device with empty ID to be removed from folder")
	}
}

func TestMaxConcurrentFolders(t *testing.T) {
	cases := []struct {
		input  int
		output int
	}{
		{input: -42, output: 0},
		{input: -1, output: 0},
		{input: 0, output: runtime.GOMAXPROCS(-1)},
		{input: 1, output: 1},
		{input: 42, output: 42},
	}

	for _, tc := range cases {
		opts := OptionsConfiguration{RawMaxFolderConcurrency: tc.input}
		res := opts.MaxFolderConcurrency()
		if res != tc.output {
			t.Errorf("Wrong MaxFolderConcurrency, %d => %d, expected %d", tc.input, res, tc.output)
		}
	}
}

// defaultConfigAsMap returns a valid default config as a JSON-decoded
// map[string]interface{}. This is useful to override random elements and
// re-encode into JSON.
func defaultConfigAsMap() map[string]interface{} {
	cfg := New(device1)
	cfg.Devices = append(cfg.Devices, NewDeviceConfiguration(device2, "name"))
	cfg.Folders = append(cfg.Folders, NewFolderConfiguration(device1, "default", "default", fs.FilesystemTypeBasic, "/tmp"))
	bs, err := json.Marshal(cfg)
	if err != nil {
		// can't happen
		panic(err)
	}
	var tmp map[string]interface{}
	if err := json.Unmarshal(bs, &tmp); err != nil {
		// can't happen
		panic(err)
	}
	return tmp
}

func load(path string, myID protocol.DeviceID) (Wrapper, error) {
	cfg, _, err := Load(path, myID, events.NoopLogger)
	return cfg, err
}

func wrap(path string, cfg Configuration) Wrapper {
	return Wrap(path, cfg, events.NoopLogger)
}

func TestInternalVersioningConfiguration(t *testing.T) {
	// Verify that the versioning configuration XML seralizes to something
	// reasonable.

	cfg := New(device1)
	cfg.Folders = append(cfg.Folders, NewFolderConfiguration(device1, "default", "default", fs.FilesystemTypeBasic, "/tmp"))
	cfg.Folders[0].Versioning = VersioningConfiguration{
		Type:             "foo",
		Params:           map[string]string{"bar": "baz"},
		CleanupIntervalS: 42,
	}

	// These things should all be present in the serialized version.
	expected := []string{
		`<versioning type="foo">`,
		`<param key="bar" val="baz"`,
		`<cleanupIntervalS>42<`,
		`</versioning>`,
	}

	bs, err := xml.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	for _, exp := range expected {
		if !strings.Contains(string(bs), exp) {
			t.Logf("%s", bs)
			t.Fatal("bad serializion of versioning parameters")
		}
	}
}
