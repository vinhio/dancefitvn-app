// Adapted from jplayer-mobile/internal/release/release_test.go at 49de3c2
// (config, parsePlist, TestPrivacyManifest's reason table).

package release_test

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"go/build"
	"image"
	_ "image/png" // the decoder of the icon
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// root is the repository root, two levels above this package.
const root = "../.."

// apple is the part of gdesk.json's ios and tvos members the gates read.
type apple struct {
	LaunchBackground        string   `json:"launchBackground"`
	UsesNonExemptEncryption *bool    `json:"usesNonExemptEncryption"`
	Localizations           []string `json:"localizations"`
	BackgroundModes         []string `json:"backgroundModes"`
	PrivacyManifest         string   `json:"privacyManifest"`
}

// config is the part of gdesk.json the release gates read.
type config struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Build   int    `json:"build"`
	Icon    string `json:"icon"`
	IOS     apple  `json:"ios"`
	TVOS    apple  `json:"tvos"`
	Android struct {
		TargetSdk      int      `json:"targetSdk"`
		CleartextHosts []string `json:"cleartextHosts"`
		Permissions    []string `json:"permissions"`
		Splash         struct {
			Background string `json:"background"`
		} `json:"splash"`
	} `json:"android"`
	Signing map[string]string `json:"signing"`
}

// plist is a value of a property list as the privacy manifests use them.
type plist any

// The members a privacy manifest may hold (Apple's four).
var manifestKeys = []string{
	"NSPrivacyTracking", "NSPrivacyTrackingDomains", "NSPrivacyCollectedDataTypes", "NSPrivacyAccessedAPITypes",
}

// reasons are the valid codes of Apple's required reason API list for the
// categories a manifest may name.
var reasons = map[string][]string{
	"FileTimestamp":   {"DDA9.1", "C617.1", "3B52.1", "0A2A.1"},
	"SystemBootTime":  {"35F9.1", "8FFB.1", "3D61.1"},
	"DiskSpace":       {"85F4.1", "E174.1", "7D9E.1", "B728.1"},
	"ActiveKeyboards": {"3EC4.1", "54BD.1"},
	"UserDefaults":    {"CA92.1", "1C8F.1", "C56D.1", "AC6B.1"},
}

// dataTypes are Apple's NSPrivacyCollectedDataType values, without the
// prefix.
var dataTypes = []string{
	"Name", "EmailAddress", "PhoneNumber", "PhysicalAddress", "OtherUserContactInfo", "Health", "Fitness",
	"PaymentInfo", "CreditInfo", "OtherFinancialInfo", "PreciseLocation", "CoarseLocation", "SensitiveInfo",
	"Contacts", "EmailsOrTextMessages", "PhotosorVideos", "AudioData", "GameplayContent", "CustomerSupport",
	"OtherUserContent", "BrowsingHistory", "SearchHistory", "UserID", "DeviceID", "PurchaseHistory",
	"ProductInteraction", "AdvertisingData", "OtherUsageData", "CrashData", "PerformanceData",
	"OtherDiagnosticData", "EnvironmentScanning", "Hands", "Head", "OtherDataTypes",
}

// manifest is what a system's privacy manifest declares (Q-P8-7, Q-P8-8).
type manifest struct {
	sys       string
	apis      map[string][]string
	collected []string
}

// manifests are the declarations of local/plans/P8.md P8.2a: the phone
// reads the disk's free space (sheet 09, screen 11), the TV keeps its
// session in NSUserDefaults (tvstore) and collects no phone, photo or
// coach profile.
var manifests = []manifest{
	{
		sys: "ios",
		apis: map[string][]string{
			"FileTimestamp":  {"C617.1"},
			"SystemBootTime": {"35F9.1"},
			"DiskSpace":      {"E174.1", "85F4.1"},
		},
		collected: []string{
			"Name", "EmailAddress", "PhoneNumber", "PhotosorVideos", "OtherUserContent", "UserID", "DeviceID",
			"ProductInteraction",
		},
	},
	{
		sys: "tvos",
		apis: map[string][]string{
			"FileTimestamp":  {"C617.1"},
			"SystemBootTime": {"35F9.1"},
			"UserDefaults":   {"CA92.1"},
		},
		collected: []string{"Name", "EmailAddress", "UserID", "DeviceID", "ProductInteraction"},
	},
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

func readFile(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err, rel)

	return b
}

func loadConfig(t *testing.T) config {
	t.Helper()
	var c config
	require.NoError(t, json.Unmarshal(readFile(t, "gdesk.json"), &c))

	return c
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestGdeskJSONRelease checks the store identity of gdesk.json: the
// version and build of the trial release (Q-P8-5), the target SDK Play
// takes (Q-P8-6), the export compliance answer, the language, background
// audio and the launch color on every system.
func TestGdeskJSONRelease(t *testing.T) {
	t.Parallel()
	c := loadConfig(t)

	assert.Equal(t, "com.jivecode.dancefitvn", c.ID)
	assert.Equal(t, "1.0.1", c.Version, "Q-P8-5")
	assert.Equal(t, 2, c.Build, "Q-P8-5: CFBundleVersion 2, versionCode 1000102")
	assert.Equal(t, 36, c.Android.TargetSdk, "Q-P8-6")
	assert.Equal(t, []string{"127.0.0.1"}, c.Android.CleartextHosts, "cleartext to the loopback alone (Q-P8-23)")
	assert.ElementsMatch(t, []string{"USE_BIOMETRIC", "ACCESS_NETWORK_STATE"}, c.Android.Permissions)
	assert.Equal(t, "#141518", c.Android.Splash.Background)
	for name, a := range map[string]apple{"ios": c.IOS, "tvos": c.TVOS} {
		require.NotNil(t, a.UsesNonExemptEncryption, "%s.usesNonExemptEncryption must be set", name)
		assert.False(t, *a.UsesNonExemptEncryption, name)
		assert.Equal(t, []string{"vi"}, a.Localizations, name)
		assert.Equal(t, []string{"audio"}, a.BackgroundModes, name)
		assert.Equal(t, "#141518", a.LaunchBackground, name)
		assert.Equal(t, "assets/"+name+"/PrivacyInfo.xcprivacy", a.PrivacyManifest, name)
		assert.FileExists(t, filepath.Join(root, a.PrivacyManifest), name)
	}
}

// TestGdeskJSONSigningHoldsNoSecret checks that a signing member, when a
// signer's Mac adds one, names files under local/ (gitignored) and no
// password: passwords stay in the variables the …Env fields name.
func TestGdeskJSONSigningHoldsNoSecret(t *testing.T) {
	t.Parallel()
	c := loadConfig(t)

	for k, v := range c.Signing {
		low := strings.ToLower(k)
		assert.NotContains(t, low, "password", "%s: passwords go in environment variables (…Env)", k)
		switch k {
		case "iosProfile", "tvosProfile", "androidKeystore":
			assert.True(t, strings.HasPrefix(v, "local/"), "%s = %s: keep it under local/", k, v)
		default:
		}
	}
}

// TestIconIsOpaque1024 checks the icon a device release takes: a square
// PNG of 1024 pixels without one transparent pixel (ITMS-90717, Q-P8-12).
func TestIconIsOpaque1024(t *testing.T) {
	t.Parallel()
	c := loadConfig(t)

	img, format, err := image.Decode(bytes.NewReader(readFile(t, c.Icon)))
	require.NoError(t, err)
	assert.Equal(t, "png", format)
	b := img.Bounds()
	require.Equal(t, 1024, b.Dx())
	require.Equal(t, 1024, b.Dy())
	clear := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				clear++
			}
		}
	}
	assert.Zero(t, clear, "pixels not opaque")
}

// TestPrivacyManifests checks both manifests: an XML property list of
// Apple's four members, no tracking, exactly the categories and reasons
// of P8.2a (each reason one Apple lists for it) and exactly the collected
// data types of Q-P8-8, each linked to the user, not for tracking, for
// the app's functionality.
func TestPrivacyManifests(t *testing.T) {
	t.Parallel()
	c := loadConfig(t)

	for _, m := range manifests {
		t.Run(m.sys, func(t *testing.T) {
			t.Parallel()
			path := c.IOS.PrivacyManifest
			if m.sys == "tvos" {
				path = c.TVOS.PrivacyManifest
			}
			raw := readFile(t, path)
			require.True(t, strings.HasPrefix(string(raw), "<?xml"), "XML plist (kits/cli checkPrivacyManifest)")
			top, ok := parsePlist(t, raw).(map[string]plist)
			require.True(t, ok)
			for k := range top {
				assert.Contains(t, manifestKeys, k, "not one of Apple's members")
			}
			assert.Equal(t, false, top["NSPrivacyTracking"], "no tracking")
			assert.Equal(t, []plist{}, top["NSPrivacyTrackingDomains"])
			assert.Equal(t, m.apis, accessed(t, top), "the categories and reasons of P8.2a")
			assert.Equal(t, m.collected, collected(t, top), "the data types of Q-P8-8")
		})
	}
}

// TestDiskSpaceFollowsTheCode keeps the DiskSpace declarations tied to
// the code: the iOS manifest declares it (both reasons) because
// internal/sys/freespace_ios.go reads the volume keys and statfs, and the
// tvOS one does not because no file the TV build compiles in internal/app
// calls sys.FreeSpace (scripts/release/check-ios.sh confirms it on the
// programs).
func TestDiskSpaceFollowsTheCode(t *testing.T) {
	t.Parallel()

	src := string(readFile(t, "internal/sys/freespace_ios.go"))
	usesVolumeKeys := strings.Contains(src, "NSURLVolumeAvailableCapacityForImportantUsageKey") &&
		strings.Contains(src, "NSURLVolumeTotalCapacityKey")
	assert.True(t, usesVolumeKeys, "freespace_ios.go names the volume keys")
	assert.Contains(t, string(readFile(t, "internal/sys/freespace_statfs.go")), "syscall.Statfs")
	assert.Equal(t, []string{"85F4.1", "E174.1"}, sorted(manifests[0].apis["DiskSpace"]), "N7: two reasons")

	ctx := build.Default
	ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = "ios", "arm64", true
	ctx.BuildTags = []string{"tvos", "live"}
	var tvCallers []string
	for _, dir := range []string{"internal/app", "internal/ui/tv"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			match, err := ctx.MatchFile(filepath.Dir(path), d.Name())
			if err != nil || !match {
				return err
			}
			b, err := os.ReadFile(path)
			if err == nil && bytes.Contains(b, []byte("sys.FreeSpace")) {
				tvCallers = append(tvCallers, path)
			}

			return err
		})
		require.NoError(t, err)
	}
	assert.Empty(t, tvCallers, "the TV build calls sys.FreeSpace: declare DiskSpace in the tvOS manifest")
	_, tvDeclares := manifests[1].apis["DiskSpace"]
	assert.False(t, tvDeclares)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// accessed returns the categories (without the prefix) and reasons of
// NSPrivacyAccessedAPITypes, checking each against Apple's list.
func accessed(t *testing.T, top map[string]plist) map[string][]string {
	t.Helper()
	types, ok := top["NSPrivacyAccessedAPITypes"].([]plist)
	require.True(t, ok)
	got := map[string][]string{}
	for _, e := range types {
		d, ok := e.(map[string]plist)
		require.True(t, ok)
		full, _ := d["NSPrivacyAccessedAPIType"].(string)
		cat := strings.TrimPrefix(full, "NSPrivacyAccessedAPICategory")
		require.Contains(t, reasons, cat, "unknown category %s", full)
		rs, _ := d["NSPrivacyAccessedAPITypeReasons"].([]plist)
		require.NotEmpty(t, rs, cat)
		_, dup := got[cat]
		assert.False(t, dup, "category %s declared twice", cat)
		for _, r := range rs {
			s, _ := r.(string)
			assert.Contains(t, reasons[cat], s, "invalid reason for %s", cat)
			got[cat] = append(got[cat], s)
		}
	}

	return got
}

// collected returns the data types (without the prefix) of
// NSPrivacyCollectedDataTypes in order, checking each entry.
func collected(t *testing.T, top map[string]plist) []string {
	t.Helper()
	entries, ok := top["NSPrivacyCollectedDataTypes"].([]plist)
	require.True(t, ok)
	var got []string
	for _, e := range entries {
		d, ok := e.(map[string]plist)
		require.True(t, ok)
		full, _ := d["NSPrivacyCollectedDataType"].(string)
		kind := strings.TrimPrefix(full, "NSPrivacyCollectedDataType")
		assert.Contains(t, dataTypes, kind, "unknown data type %s", full)
		assert.NotContains(t, got, kind, "listed twice")
		assert.Equal(t, true, d["NSPrivacyCollectedDataTypeLinked"], "%s: linked to the account", kind)
		assert.Equal(t, false, d["NSPrivacyCollectedDataTypeTracking"], "%s: no tracking", kind)
		assert.Equal(t, []plist{"NSPrivacyCollectedDataTypePurposeAppFunctionality"}, d["NSPrivacyCollectedDataTypePurposes"], kind)
		assert.Len(t, d, 4, "%s: the four keys of an entry", kind)
		got = append(got, kind)
	}

	return got
}

// sorted returns a sorted copy of s.
func sorted(s []string) []string {
	c := slices.Clone(s)
	sort.Strings(c)

	return c
}

// parsePlist reads the XML plist subset the manifests use: dict, array,
// string, true, false.
func parsePlist(t *testing.T, data []byte) plist {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.Entity = xml.HTMLEntity
	var value func(start xml.StartElement) plist
	value = func(start xml.StartElement) plist {
		switch start.Name.Local {
		case "true":
			require.NoError(t, dec.Skip())
			return true
		case "false":
			require.NoError(t, dec.Skip())
			return false
		case "string":
			var s string
			require.NoError(t, dec.DecodeElement(&s, &start))
			return s
		case "array":
			arr := []plist{}
			for {
				tok, err := dec.Token()
				require.NoError(t, err)
				switch e := tok.(type) {
				case xml.StartElement:
					arr = append(arr, value(e))
				case xml.EndElement:
					return arr
				default:
				}
			}
		case "dict":
			d := map[string]plist{}
			key := ""
			for {
				tok, err := dec.Token()
				require.NoError(t, err)
				switch e := tok.(type) {
				case xml.StartElement:
					if e.Name.Local == "key" {
						require.NoError(t, dec.DecodeElement(&key, &e))
						continue
					}
					require.NotEmpty(t, key, "value without key")
					d[key] = value(e)
					key = ""
				case xml.EndElement:
					return d
				default:
				}
			}
		default:
			t.Fatalf("unsupported plist element %q", start.Name.Local)
			return nil
		}
	}
	for {
		tok, err := dec.Token()
		require.NoError(t, err, "no plist root")
		if s, ok := tok.(xml.StartElement); ok && s.Name.Local == "dict" {
			return value(s)
		}
	}
}
