package model

import (
	"math"
	"strings"

	"github.com/howeyc/ledger"
	"github.com/jbrukh/bayesian"
)

type Model interface {
	Train(generalLedger []*ledger.Transaction, matchingAccount string)
	Predict(inputPayeeWords []string) string
}

type BayesianModel struct {
	classifier *bayesian.Classifier
}

func (m *BayesianModel) Train(generalLedger []*ledger.Transaction, matchingAccount string) {
	allAccounts := ledger.GetBalances(generalLedger, []string{})
	uniqueAccounts := make(map[string]bool)
	for _, acc := range allAccounts {
		if ok, _ := uniqueAccounts[acc.Name]; !ok {
			uniqueAccounts[acc.Name] = true
		}
	}

	classes := []bayesian.Class{}
	for name := range uniqueAccounts {
		classes = append(classes, bayesian.Class(name))
	}

	classifier := bayesian.NewClassifier(classes...)
	for _, tran := range generalLedger {
		payeeWords := strings.Fields(tran.Payee)
		// learn accounts names (except matchingAccount) for transactions where matchingAccount is present
		learnName := false
		for _, accChange := range tran.AccountChanges {
			if accChange.Name == matchingAccount {
				learnName = true
				break
			}
		}
		if learnName {
			for _, accChange := range tran.AccountChanges {
				if accChange.Name != matchingAccount {
					classifier.Learn(payeeWords, bayesian.Class(accChange.Name))
				}
			}
		}
	}

	m.classifier = classifier
}

func (m *BayesianModel) Predict(inputPayeeWords []string) string {
	if m.classifier == nil {
		return "unknown:unknown"
	}

	// Classify into expense account

	// Find the highest and second highest scores
	highScore1 := math.Inf(-1)
	highScore2 := math.Inf(-1)
	matchIdx := 0
	scores, _, _ := m.classifier.LogScores(inputPayeeWords)
	for j, score := range scores {
		if score > highScore1 {
			highScore2 = highScore1
			highScore1 = score
			matchIdx = j
		}
	}
	// If the difference between the highest and second highest scores is greater than 10
	// then it indicates that highscore is a high confidence match
	if highScore1-highScore2 > 10 {
		return string(m.classifier.Classes[matchIdx])
	} else {
		return "unknown:unknown"
	}
}
