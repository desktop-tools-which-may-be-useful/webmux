// Command releasesign creates the release signing keys, signs SHA256SUMS
// and verifies the signature against the keys built into WebMux.
//
//	releasesign keygen -secret FILE          write a new key; print the public key
//	releasesign sign -version V FILE         sign FILE into FILE.minisig
//	releasesign verify [-version V] FILE     verify FILE.minisig with built-in keys
//
// sign reads the key from $WEBMUX_RELEASE_SIGNING_KEY for a release version
// and from $WEBMUX_DEV_SIGNING_KEY for a MAJOR.MINOR.PATCH-dev.N development
// build, and never writes it. verify accepts a signature only from the key
// group matching its version.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/rainchan/webmux/internal/update"
)

const (
	keyEnv    = "WEBMUX_RELEASE_SIGNING_KEY"
	devKeyEnv = "WEBMUX_DEV_SIGNING_KEY"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "releasesign:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: releasesign keygen|sign|verify")
	}
	switch args[0] {
	case "keygen":
		return keygen(args[1:], stdout)
	case "sign":
		return sign(args[1:])
	case "verify":
		return verify(args[1:], stdout)
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func keygen(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("keygen", flag.ContinueOnError)
	secret := flags.String("secret", "", "file to create with the secret key (mode 0600)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *secret == "" {
		return errors.New("keygen needs -secret FILE")
	}
	key, err := update.GenerateKey()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(*secret, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, key.String()); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_, err = fmt.Fprint(stdout, key.Public().File())
	return err
}

func sign(args []string) error {
	flags := flag.NewFlagSet("sign", flag.ContinueOnError)
	version := flags.String("version", "", "version, MAJOR.MINOR.PATCH or MAJOR.MINOR.PATCH-dev.N")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("sign needs exactly one file")
	}
	v, ok := update.ParseVersion(*version)
	if !ok {
		return fmt.Errorf("-version %q is not MAJOR.MINOR.PATCH or MAJOR.MINOR.PATCH-dev.N", *version)
	}
	env := keyEnv
	if v.IsDev() {
		env = devKeyEnv
	}
	text := os.Getenv(env)
	if text == "" {
		return fmt.Errorf("$%s is not set", env)
	}
	key, err := update.ParseSecretKey(text)
	if err != nil {
		return err
	}
	path := flags.Arg(0)
	message, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sig, err := key.Sign(message, "webmux "+*version)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".minisig", sig, 0o644)
}

func verify(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	version := flags.String("version", "", "require this version in the trusted comment")
	pubkey := flags.String("pubkey", "", "verify with this public key, in the group -version selects, instead of the built-in ones")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("verify needs exactly one file")
	}
	stable, dev := update.StableKeys(), update.DevKeys()
	if *pubkey != "" {
		k, err := update.ParsePublicKey(*pubkey)
		if err != nil {
			return err
		}
		stable, dev = []update.PublicKey{k}, nil
		if update.IsDevVersion(*version) {
			stable, dev = nil, []update.PublicKey{k}
		}
	}
	path := flags.Arg(0)
	message, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(path + ".minisig")
	if err != nil {
		return err
	}
	m, err := update.VerifyManifest(message, sig, stable, dev)
	if err != nil {
		return err
	}
	if *version != "" && m.Version != *version {
		return fmt.Errorf("signature is for webmux %s, not %s", m.Version, *version)
	}
	_, err = fmt.Fprintf(stdout, "%s: signature OK (webmux %s)\n", path, m.Version)
	return err
}
