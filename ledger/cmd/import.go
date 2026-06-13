package cmd

import (
	"errors"

	"github.com/howeyc/ledger"
	"github.com/spf13/cobra"
)

var (
	ErrNoMatchingAccount = errors.New("Unable to find matching account.")
)

var csvDateFormat string
var negateAmount bool
var allowMatching bool
var fieldDelimiter string
var scaleFactor float64
var overrideCurrency string

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import <account-substring> <csv-file>",
	Args:  cobra.ExactArgs(2),
	Short: "Import transactions from csv to ledger format",
	Run: func(_ *cobra.Command, args []string) {
		accountSubstring := args[0]
		fileName := args[1]

		imp := ledger.NewImporter(accountSubstring, fileName, fieldDelimiter, ledgerFilePath, scaleFactor, overrideCurrency)
		defer imp.Close()

		ledger := imp.Import()

		PrintLedger(ledger, []string{}, 80)
	},
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().BoolVar(&negateAmount, "neg", false, "Negate amount column value.")
	importCmd.Flags().BoolVar(&allowMatching, "allow-matching", false, "Have output include imported transactions that\nmatch existing ledger transactions.")
	importCmd.Flags().Float64Var(&scaleFactor, "scale", 1.0, "Scale factor to multiply against every imported amount.")
	importCmd.Flags().StringVar(&csvDateFormat, "date-format", "01/02/2006", "Date format.")
	importCmd.Flags().StringVar(&fieldDelimiter, "delimiter", ",", "Field delimiter.")
	importCmd.Flags().StringVar(&overrideCurrency, "override-currency", "", "Override detected currency for imported transactions.")
}
