// Package demo is the fictional company used by the tests, the tutorial and
// the demonstration: the same synthetic data, spread over several sources.
//
// The data is generated from a fixed seed, so it is always the same. It is
// wrong on purpose in the ways real company data is wrong, because that is
// what tests whether unifying it works:
//
//   - the same customer is registered twice in the CRM with different
//     spellings, and the spreadsheet spells customer names its own way;
//   - the keys do not match: the CRM uses the number 42, the spreadsheet the
//     code "C-0042", and some codes in the spreadsheet exist nowhere else;
//   - dates come as a date in the CRM, as "03/2026" or "2026-03" in the
//     spreadsheet and as "01/01/2026" in the PDF;
//   - one salesperson is spelled without accents in the PDF.
//
// To add a source, add its data to Company, fill it in Generate and write one
// more writer next to sql.go, xlsx.go and pdf.go.
package demo

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

// Company is all the data of the fictional company, before it is split into
// sources.
type Company struct {
	Salespeople []Salesperson
	Customers   []Customer
	Targets     []Target
}

// Salesperson appears in the commission table (PDF) and, by name, in the CRM
// and in the spreadsheet.
type Salesperson struct {
	Name string
	// PDFName is how the commission table spells the name.
	PDFName    string
	Region     string
	Commission float64 // percent
	Since      time.Time
}

// Customer is a row of the CRM (Postgres).
type Customer struct {
	ID          int
	Name        string
	CNPJ        string
	City        string
	State       string
	Segment     string
	Salesperson string
	CreatedAt   time.Time
}

// Target is a row of the sales targets spreadsheet (Excel).
type Target struct {
	CustomerCode string // "C-0042"
	CustomerName string
	Salesperson  string
	Month        string // "03/2026", sometimes "2026-03"
	Amount       float64
}

const (
	seed          = 20261113
	customerCount = 150
	// duplicateCount customers are registered a second time, spelled differently.
	duplicateCount = 6
	// orphanCount spreadsheet rows point to customer codes that do not exist.
	orphanCount = 5
)

var (
	salespeople = []struct{ name, pdfName, region string }{
		{"Ana Beatriz Souza", "", "Sul"},
		{"Carlos Eduardo Lima", "", "Sudeste"},
		{"Fernanda Oliveira", "", "Sudeste"},
		{"João Pedro Almeida", "Joao Pedro Almeida", "Nordeste"},
		{"Luís Antônio Prado", "", "Centro-Oeste"},
		{"Mariana Costa", "", "Sul"},
		{"Rafael Nogueira", "", "Norte"},
		{"Tatiane Ribeiro", "", "Nordeste"},
	}
	// citiesByRegion keeps customers in the region of their salesperson.
	citiesByRegion = map[string][]struct{ city, state string }{
		"Sul":          {{"Curitiba", "PR"}, {"Porto Alegre", "RS"}, {"Florianópolis", "SC"}, {"Londrina", "PR"}},
		"Sudeste":      {{"São Paulo", "SP"}, {"Campinas", "SP"}, {"Belo Horizonte", "MG"}, {"Rio de Janeiro", "RJ"}},
		"Nordeste":     {{"Recife", "PE"}, {"Salvador", "BA"}, {"Fortaleza", "CE"}},
		"Centro-Oeste": {{"Goiânia", "GO"}, {"Cuiabá", "MT"}, {"Campo Grande", "MS"}},
		"Norte":        {{"Manaus", "AM"}, {"Belém", "PA"}},
	}
	businesses = []struct{ prefix, segment string }{
		{"Comercial", "Varejo"},
		{"Mercado", "Varejo"},
		{"Distribuidora", "Atacado"},
		{"Atacadão", "Atacado"},
		{"Indústria", "Indústria"},
		{"Metalúrgica", "Indústria"},
		{"Transportes", "Logística"},
		{"Farmácia", "Saúde"},
		{"Clínica", "Saúde"},
		{"Auto Peças", "Automotivo"},
	}
	surnames = []string{
		"Andrade", "Barbosa", "Cardoso", "Dias", "Esteves", "Ferreira", "Gonçalves", "Hoffmann",
		"Ishikawa", "Junqueira", "Klein", "Lacerda", "Machado", "Novaes", "Orsini", "Pacheco",
		"Queiroz", "Rezende", "Schmidt", "Teixeira", "Uchôa", "Vasconcelos", "Weber", "Xavier",
		"Yamamoto", "Zanetti", "Bragança", "Conceição", "Guimarães", "Tavares",
	}
	companyTypes = []string{"Ltda", "S.A.", "ME", "EIRELI"}
	months       = []time.Month{time.January, time.February, time.March, time.April, time.May, time.June}
)

const targetYear = 2026

// Generate builds the fictional company. It always returns the same data.
func Generate() Company {
	// PCG is specified, so the sequence is the same on every platform and Go
	// version, unlike the default source.
	rng := rand.New(rand.NewPCG(seed, seed))
	var c Company

	for i, s := range salespeople {
		pdfName := s.pdfName
		if pdfName == "" {
			pdfName = s.name
		}
		c.Salespeople = append(c.Salespeople, Salesperson{
			Name:       s.name,
			PDFName:    pdfName,
			Region:     s.region,
			Commission: 2.5 + 0.5*float64(i%5),
			Since:      time.Date(targetYear, time.January, 1, 0, 0, 0, 0, time.UTC),
		})
	}

	used := map[string]bool{}
	for len(c.Customers) < customerCount-duplicateCount {
		business := pick(rng, businesses)
		name := fmt.Sprintf("%s %s %s", business.prefix, pick(rng, surnames), pick(rng, companyTypes))
		if used[name] {
			continue
		}
		used[name] = true
		seller := pick(rng, c.Salespeople)
		place := pick(rng, citiesByRegion[seller.Region])
		c.Customers = append(c.Customers, Customer{
			ID:          len(c.Customers) + 1,
			Name:        name,
			CNPJ:        cnpj(rng),
			City:        place.city,
			State:       place.state,
			Segment:     business.segment,
			Salesperson: seller.Name,
			CreatedAt:   time.Date(2019+rng.IntN(7), time.Month(1+rng.IntN(12)), 1+rng.IntN(28), 0, 0, 0, 0, time.UTC),
		})
	}
	// The same customer registered again later, by someone who typed the name
	// in capitals and the CNPJ without punctuation.
	for i := range duplicateCount {
		original := c.Customers[i*17]
		duplicate := original
		duplicate.ID = len(c.Customers) + 1
		duplicate.Name = strings.ToUpper(original.Name) + "."
		duplicate.CNPJ = digitsOnly(original.CNPJ)
		duplicate.CreatedAt = original.CreatedAt.AddDate(1, 2, 3)
		c.Customers = append(c.Customers, duplicate)
	}

	for _, month := range months {
		for _, customer := range c.Customers {
			if rng.IntN(5) == 0 { // not every customer has a target every month
				continue
			}
			c.Targets = append(c.Targets, Target{
				CustomerCode: fmt.Sprintf("C-%04d", customer.ID),
				CustomerName: spreadsheetName(rng, customer.Name),
				Salesperson:  customer.Salesperson,
				Month:        spreadsheetMonth(rng, month),
				Amount:       float64(5+rng.IntN(196)) * 500,
			})
		}
	}
	for i := range orphanCount {
		c.Targets = append(c.Targets, Target{
			CustomerCode: fmt.Sprintf("C-%04d", 9000+i),
			CustomerName: "Cliente em prospecção",
			Salesperson:  pick(rng, c.Salespeople).Name,
			Month:        spreadsheetMonth(rng, months[i%len(months)]),
			Amount:       float64(5+rng.IntN(196)) * 500,
		})
	}
	return c
}

func pick[T any](rng *rand.Rand, items []T) T {
	return items[rng.IntN(len(items))]
}

// cnpj returns a number shaped like a CNPJ. The check digits are random, so it
// is not a valid one and cannot belong to a real company.
func cnpj(rng *rand.Rand) string {
	return fmt.Sprintf("%02d.%03d.%03d/0001-%02d", rng.IntN(100), rng.IntN(1000), rng.IntN(1000), rng.IntN(100))
}

func digitsOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if r < '0' || r > '9' {
			return -1
		}
		return r
	}, s)
}

// spreadsheetName spells a customer name the way whoever keeps the spreadsheet
// would: usually right, sometimes in capitals, sometimes without the company
// type, sometimes with stray spaces.
func spreadsheetName(rng *rand.Rand, name string) string {
	switch rng.IntN(10) {
	case 0:
		return strings.ToUpper(name)
	case 1:
		return name[:strings.LastIndex(name, " ")]
	case 2:
		return " " + name + "  "
	default:
		return name
	}
}

func spreadsheetMonth(rng *rand.Rand, month time.Month) string {
	if rng.IntN(10) == 0 {
		return fmt.Sprintf("%d-%02d", targetYear, int(month))
	}
	return fmt.Sprintf("%02d/%d", int(month), targetYear)
}
