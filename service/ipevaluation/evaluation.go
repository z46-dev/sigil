//go:build !js

package ipevaluation

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

type (
	// Result contains network ownership and coarse location data for an IP address.
	Result struct {
		ASN             uint
		ASNOrganization string
		CountryCode     string
		City            string
		OpenProxy       bool
		Tor             bool
		Hosting         bool
		Malicious       bool
	}

	// Evaluator owns reusable in-memory MaxMind database readers.
	Evaluator struct {
		asn     *maxminddb.Reader
		city    *maxminddb.Reader
		country *maxminddb.Reader
		feeds   *feedService
	}

	asnRecord struct {
		Number       uint   `maxminddb:"autonomous_system_number"`
		Organization string `maxminddb:"autonomous_system_organization"`
	}

	locationRecord struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
		RegisteredCountry struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"registered_country"`
		City struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"city"`
	}
)

const maximumDatabaseBytes int64 = 128 * 1024 * 1024

var active *Evaluator

func archiveFor(directory, database string) (path string, err error) {
	var matches []string

	if matches, err = filepath.Glob(filepath.Join(directory, database+"_*.tar.gz")); err != nil {
		return
	}

	if len(matches) == 0 {
		err = fmt.Errorf("no %s archive found in %s", database, directory)
		return
	}

	sort.Strings(matches)
	path = matches[len(matches)-1]
	return
}

func databaseFromArchive(directory, path, databaseType string) (reader *maxminddb.Reader, err error) {
	var root *os.Root
	if root, err = os.OpenRoot(directory); err != nil {
		err = fmt.Errorf("open archive directory %s: %w", directory, err)
		return
	}
	defer root.Close()

	var archive *os.File
	if archive, err = root.Open(filepath.Base(path)); err != nil {
		err = fmt.Errorf("open %s: %w", path, err)
		return
	}
	defer archive.Close()

	var compressed *gzip.Reader
	if compressed, err = gzip.NewReader(archive); err != nil {
		err = fmt.Errorf("read gzip %s: %w", path, err)
		return
	}
	defer compressed.Close()

	var entries *tar.Reader = tar.NewReader(compressed)
	for {
		var header *tar.Header
		if header, err = entries.Next(); err != nil {
			if err == io.EOF {
				err = fmt.Errorf("%s does not contain an MMDB database", path)
			}
			return
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		if header.Size > maximumDatabaseBytes {
			err = fmt.Errorf("archive member in %s has invalid size %d", path, header.Size)
			return
		}

		if !strings.EqualFold(filepath.Ext(header.Name), ".mmdb") {
			continue
		}

		if header.Size <= 0 {
			err = fmt.Errorf("MMDB member in %s is empty", path)
			return
		}

		var contents []byte = make([]byte, header.Size)
		if _, err = io.ReadFull(entries, contents); err != nil {
			err = fmt.Errorf("read MMDB member from %s: %w", path, err)
			return
		}

		if reader, err = maxminddb.OpenBytes(contents); err != nil {
			err = fmt.Errorf("open MMDB member from %s: %w", path, err)
			return
		}

		if reader.Metadata.DatabaseType != databaseType {
			var actualType string = reader.Metadata.DatabaseType
			_ = reader.Close()
			reader = nil
			err = fmt.Errorf("archive %s contains %q, expected %q", path, actualType, databaseType)
		}

		return
	}
}

func openDatabase(directory, databaseType string) (reader *maxminddb.Reader, err error) {
	var path string
	if path, err = archiveFor(directory, databaseType); err == nil {
		reader, err = databaseFromArchive(directory, path, databaseType)
	}

	return
}

// Open loads GeoLite archives into memory without extracting files to disk.
func Open(directory string) (evaluator *Evaluator, err error) {
	evaluator = &Evaluator{}
	if evaluator.asn, err = openDatabase(directory, "GeoLite2-ASN"); err != nil {
		return
	}

	if evaluator.city, err = openDatabase(directory, "GeoLite2-City"); err != nil {
		evaluator.Close()
		return
	}

	if evaluator.country, err = openDatabase(directory, "GeoLite2-Country"); err != nil {
		evaluator.Close()
	}

	return
}

// Evaluate looks up network ownership and location without retaining the address.
func (evaluator *Evaluator) Evaluate(address string) (result Result, err error) {
	var ip netip.Addr
	if ip, err = netip.ParseAddr(address); err != nil {
		err = fmt.Errorf("parse IP address: %w", err)
		return
	}
	ip = ip.Unmap()

	var asn asnRecord
	if err = evaluator.asn.Lookup(ip).Decode(&asn); err != nil {
		return
	}
	result.ASN = asn.Number
	result.ASNOrganization = asn.Organization

	var location locationRecord
	if err = evaluator.city.Lookup(ip).Decode(&location); err != nil {
		return
	}
	result.CountryCode = location.Country.ISOCode
	if result.CountryCode == "" {
		result.CountryCode = location.RegisteredCountry.ISOCode
	}
	result.City = location.City.Names["en"]

	if result.CountryCode == "" {
		if err = evaluator.country.Lookup(ip).Decode(&location); err != nil {
			return
		}
		result.CountryCode = location.Country.ISOCode
		if result.CountryCode == "" {
			result.CountryCode = location.RegisteredCountry.ISOCode
		}
	}

	if evaluator.feeds != nil {
		var intelligence Intelligence = evaluator.feeds.index.Load().evaluate(ip)
		result.OpenProxy = intelligence.OpenProxy
		result.Tor = intelligence.Tor
		result.Hosting = intelligence.Hosting
		result.Malicious = intelligence.Malicious
	}

	return
}

// Close releases all in-memory database readers.
func (evaluator *Evaluator) Close() {
	if evaluator == nil {
		return
	}

	for _, reader := range []*maxminddb.Reader{evaluator.asn, evaluator.city, evaluator.country} {
		if reader != nil {
			_ = reader.Close()
		}
	}

	evaluator.feeds.close()
}

// Init installs the process-wide evaluator used by request handlers.
func Init(directory, intelligenceDatabase, threatFoxAuthKey string) (err error) {
	active, err = Open(directory)
	if err != nil {
		return
	}

	if active.feeds, err = startFeedService(intelligenceDatabase, threatFoxAuthKey); err != nil {
		active.Close()
		active = nil
	}
	return
}

// Evaluate uses the initialized process-wide evaluator.
func Evaluate(address string) (result Result, err error) {
	if active == nil {
		err = fmt.Errorf("IP evaluator is not initialized")
		return
	}

	result, err = active.Evaluate(address)
	return
}

// Close releases the process-wide evaluator.
func Close() {
	active.Close()
	active = nil
}
