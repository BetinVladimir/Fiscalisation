// dossier-export never invokes persistence.Open or domain exports (both may write).
package main

import (
	"context"
	"encoding/json"
	"fiscalisation/fiscal-backend/internal/dossier"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"time"
)

func main() {
	profile := flag.String("profile", "fiscal", "explicit independent profile: fiscal, minipos or fiscal-evidence")
	verify := flag.String("verify", "", "verify archive inventory/hashes without DB access")
	schema := flag.String("schema", "fiscal", "explicit deployed schema: fiscal, minipos or public")
	tenant := flag.String("tenant", "", "tenant ID")
	output := flag.String("output", "", "new output directory/file")
	contract := flag.String("contract", "", "reviewed dictionary JSON")
	source := flag.String("source-commit", "", "captured application source commit")
	inspect := flag.Bool("dictionary", false, "generate dictionary ONLY on a clean migration fixture")
	flag.Parse()
	if *verify != "" {
		if err := dossier.Verify(*verify); err != nil {
			fmt.Fprintln(os.Stderr, "Verification refused:", err)
			os.Exit(1)
		}
		fmt.Println("ARCHIVE_INTEGRITY_VERIFIED; provenance not authenticated")
		return
	}
	if err := run(*profile, *schema, *tenant, *output, *contract, *source, *inspect); err != nil {
		fmt.Fprintln(os.Stderr, "Export refused:", err)
		os.Exit(1)
	}
}
func run(profile, schema, tenant, output, contract, source string, inspect bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, os.Getenv("DOSSIER_DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("database connection failed (DSN omitted)")
	}
	defer conn.Close(ctx)
	if inspect {
		tx, e := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		value, e := dossier.InspectProfile(ctx, tx, schema, profile)
		if e != nil {
			return e
		}
		body, e := json.MarshalIndent(value, "", "  ")
		if e != nil {
			return e
		}
		file, e := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		_, e = file.Write(append(body, '\n'))
		closeErr := file.Close()
		if e != nil {
			return e
		}
		return closeErr
	}
	var value dossier.Contract
	raw, err := os.ReadFile(contract)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &value); err != nil {
		return err
	}
	return dossier.Export(ctx, conn, dossier.Options{Profile: profile, Schema: schema, Tenant: tenant, Output: output, SourceCommit: source, Contract: value})
}
