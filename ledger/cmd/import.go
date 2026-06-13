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

		// Explicitly construct the Importer struct
		imp := &ledger.Importer{
			ImportFilename:   fileName,
			DecScale:         decimal.NewFromFloat(scaleFactor),
			OverrideCurrency: overrideCurrency,
			FieldDelimiter:   fieldDelimiter,
		}

		// Replicate NewImporter setup logic
		fileReader, err := os.Open(fileName)
		if err != nil {
			fmt.Println("CSV: ", err)
			return
		}
		imp.reader = fileReader

		// If a ledger file path is provided, load it and train the classifier.
		if ledgerFilePath != "" {
			var parseError error
			imp.trainingLedger, parseError = ledger.ParseLedgerFile(ledgerFilePath)
			if parseError != nil {
				fmt.Printf("%s:%s\n", ledgerFilePath, parseError.Error())
				imp.reader.Close()
				return
			}

			matchingAccount, err := imp.findMatchingAccount(accountSubstring)
			if err != nil {
				fmt.Println(err)
				imp.reader.Close()
				return
			}
			imp.MatchingAccount = matchingAccount

			imp.classifier = imp.trainClassifier(imp.trainingLedger, imp.MatchingAccount)
		} else {
			imp.MatchingAccount = accountSubstring
		}

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
