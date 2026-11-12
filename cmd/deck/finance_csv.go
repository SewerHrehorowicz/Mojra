package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	csvQuoteChars   = ",\"\n"
	csvDoubleQuote  = `"`
	csvEscapedQuote = `""`

	fmtCashYearMonth = "%s zł (%s zł/m)"
	fmtCurrencyPLN   = "%s zł"
	fmtDividendPLN   = "%s zł dywidendy"
	fmtTotal3YPLN    = "%s zł łącznie w 3 lata"

	fmtRow0ALabel   = "A. Ty — Główny Architekt (%d%% udziałów): Faktura B2B R&D + Dywidenda"
	fmtRow0ADetail  = "Gotówka 3 lata: %s zł + wycena %d%%: %s–%s mln zł"
	fmtRow0BLabel   = "B. Przyjaciel — Co-Founder Biznes/BD (%d%% udziałów): Retainer + 15%% Prowizji B2B + Dywidenda"
	fmtRow0BDetail  = "Gotówka 3 lata: %s zł + wycena %d%% (vesting 36M): %s–%s mln zł"
	fmtRow0CLabel   = "C. Inwestorzy Seed (%d%% udziałów za %s zł)"
	fmtRow0CDetail  = "Dywidenda 3 lata: %s zł + wycena %d%%: %s–%s mln zł (zwrot %s×–%s×)"
	fmtRow1NonDil   = "%s zł bez rozwodnienia udziałów"
	fmtRow1BShares  = "%d%% udziałów (Zespół zachowuje %d%%!)"
	fmtRow3NetSplit = "%s zł (podział: %d%% Architekt, %d%% Biznes/BD, %d%% Inwestorzy)"

	simYearSummaryDesc = "Podsumowanie roczne"
	simTotal3YLabel    = "SUMA 3 LATA (M01-M36)"
	simTotal3YDesc     = "Łącznie przez 36 miesięcy"
)

type calcCSVRowTemplate struct {
	c1, c2, c3, c4, c5 string
}

type calcBudgetRowSpec struct {
	label     string
	pctCol    string
	col4      string
	detail    string
	teamShare bool
	amount    func(financeConfig) float64
}

type calcPLRowSpec struct {
	label   string
	detail  string
	sum3Y   bool
	netDesc bool
	val     func(deckYearCalc) float64
}

var (
	calcCSVHeaderRow = calcCSVRowTemplate{
		c1: "Sekcja / Pozycja",
		c2: "Rok 1",
		c3: "Rok 2",
		c4: "Rok 3",
		c5: "Suma 3 Lat / Szczegóły (Źródło: docs/finanse.cfg)",
	}

	calcSection0Header = calcCSVRowTemplate{
		c1: "0. PODZIAŁ TORTU — KTO ILE DOSTAJE (GOTÓWKA + WYCENA UDZIAŁÓW)",
		c2: "Rok 1",
		c3: "Rok 2",
		c4: "Rok 3",
		c5: "Pełna zgodność z Deckiem Inwestorskim i symulacją M1–M36",
	}

	calcSection1Header = calcCSVRowTemplate{
		c1: "1. ŹRÓDŁA BUDŻETU STARTOWEGO",
		c3: "100% budżetu",
		c5: "4 niezależne źródła finansowania",
	}

	calcSection1Rows = [...]calcBudgetRowSpec{
		{
			label:  "A. Dotacje rządowe (CRPK / MKiDN / PARP Dostępność)",
			pctCol: "33% budżetu",
			col4:   "0% oddanych udziałów",
			detail: "Granty na narzędzia kreatywne, R&D i dostępność audio (TRL 7)",
			amount: func(c financeConfig) float64 { return c.srcGrants },
		},
		{
			label:     "B. Inwestorzy Seed (2 bilety po 40 tys. zł)",
			pctCol:    "30% budżetu",
			detail:    "2 aniołów biznesu po 40 tys. zł (5% każdy)",
			teamShare: true,
			amount:    func(c financeConfig) float64 { return c.srcInvestors },
		},
		{
			label:  "C. Przedsprzedaż po darmowych Pilotach (Wspieram.to)",
			pctCol: "26% budżetu",
			col4:   "0% oddanych udziałów",
			detail: "Przedsprzedaż 4 premier po wypuszczeniu darmowych 1h Pilotów",
			amount: func(c financeConfig) float64 { return c.srcCrowdfunding },
		},
		{
			label:  "D. Pierwsze wdrożenia uczelniane B2B",
			pctCol: "11% budżetu",
			col4:   "0% oddanych udziałów",
			detail: "Pierwsze 4 szkoły filmowe / uczelnie × 7 500 zł",
			amount: func(c financeConfig) float64 { return c.srcOwnSales },
		},
	}

	calcSection2Header = calcCSVRowTemplate{
		c1: "2. KOSZTORYS STARTOWY CAPEX",
		c3: "100% wydatków",
		c4: "12 miesięcy",
		c5: "Produkcja Pilot-First (150 tys.) + R&D/Audyt/Mobile (60 tys.) + Dystrybucja/B2B (60 tys.)",
	}

	calcSection2Rows = [...]calcBudgetRowSpec{
		{
			label:  "A. Produkcja 4 własnych premier Mojra Originals (Pilot-First)",
			pctCol: "56%",
			col4:   "4 tytuły etapowane Pilotami",
			detail: "Aktorzy (60 tys.) + Studio (40 tys.) + Autorzy (34 tys.) + Ilustracje/SFX (16 tys.)",
			amount: func(c financeConfig) float64 { return c.costActors + c.costSoundStudio + c.costAuthors + c.costIllustSFX },
		},
		{
			label:  "B. R&D Aplikacji: Architektura Mobile/Steam, Audyt i Płatności",
			pctCol: "22%",
			col4:   "Gotowy silnik Go/WebGL",
			detail: "Pentest i certyfikaty (18 tys.) + R&D iOS/Android (24 tys.) + Płatności i umowy (18 tys.)",
			amount: func(c financeConfig) float64 { return c.costAuditCert + c.costMobileDev + c.costShopInfra + c.costLegal },
		},
		{
			label:  "C. Kampania Wspieram.to, promocja Steam/Mobile i akwizycja B2B",
			pctCol: "22%",
			col4:   "4 kampanie + uczelnie B2B",
			detail: "Wideo CF (18 tys.) + Promocja premier (24 tys.) + Spotkania B2B i targi (18 tys.)",
			amount: func(c financeConfig) float64 {
				return c.costCampaignCF + c.costPromoPremieres + c.costWorkshopsB2B + c.costFairsContests
			},
		},
	}

	calcSection3Header = calcCSVRowTemplate{
		c1: "3. RACHUNEK ZYSKÓW I STRAT SPÓŁKI (P&L ROK 1–3)",
		c2: "Rok 1",
		c3: "Rok 2",
		c4: "Rok 3",
		c5: "Suma 3 Lat / Opis pozycji",
	}

	calcSection3Rows = [...]calcPLRowSpec{
		{
			label:  "Przychód 1: Własne produkcje Mojra Originals (32 zł marży/szt.)",
			detail: "Wydania własne na Steam, App Store, Google Play i Wspieram.to",
			val:    func(y deckYearCalc) float64 { return y.revOrig },
		},
		{
			label:  "Przychód 2: Katalog zewnętrznych autorów (6 zł prowizji/szt.)",
			detail: "15% prowizji platformowej od zewnętrznych twórców",
			val:    func(y deckYearCalc) float64 { return y.revCat },
		},
		{
			label:  "Przychód 3: Licencje (Uczelnie B2B 7 500 zł + Indie 249 zł + Pro 599 zł)",
			detail: "Licencje kampusowe dla szkół filmowych oraz subskrypcje Playbook Studio",
			val:    func(y deckYearCalc) float64 { return y.revLic },
		},
		{
			label: "PRZYCHÓD SPÓŁKI OGÓŁEM",
			sum3Y: true,
			val:   func(y deckYearCalc) float64 { return y.revTotal },
		},
		{
			label:  "Koszt OPEX 1: R&D, architektura silnika i rozwój wieloplatformowy (B2B Architekt)",
			detail: "Kontrakt B2B Głównego Architekta (IP Box 5%) ujęty w kosztach rozwoju oprogramowania",
			val:    func(y deckYearCalc) float64 { return y.rdArchYearly },
		},
		{
			label:  "Koszt OPEX 2: Rozwój biznesu B2B (Retainer BD + 15% prowizji) oraz księgowość",
			detail: "Wynagrodzenie i prowizje Co-Foundera Biznesowego (BD) + biuro rachunkowe (2 000 zł/m)",
			val:    func(y deckYearCalc) float64 { return y.bdAndAdminYearly },
		},
		{
			label: "ZYSK OPERACYJNY SPÓŁKI (EBITDA)",
			sum3Y: true,
			val:   func(y deckYearCalc) float64 { return y.ebitdaCompany },
		},
		{
			label:   "ZYSK NETTO PO PODATKU CIT (DO PODZIAŁU DYWIDENDOWEGO)",
			netDesc: true,
			val:     func(y deckYearCalc) float64 { return y.netProfit },
		},
	}

	simCSVHeaders = [...]string{
		"Miesiąc",
		"Kamień milowy / Zdarzenie",
		"Sprzedaż Originals (szt.)",
		"Przychód Originals (zł)",
		"Przychód Licencje B2B/Studio (zł)",
		"Przychód Katalog 15% (zł)",
		"PRZYCHÓD OGÓŁEM (zł)",
		"OPEX: Faktura R&D Architekta (zł)",
		"OPEX: Przyjaciel BD + Prowizja 15% (zł)",
		"OPEX: Księgowość i biuro (zł)",
		"Zysk EBITDA Spółki (zł)",
		"Zysk Netto po CIT (zł)",
		"TORT — TY (Faktura R&D + 78% Zysku) (zł)",
		"TORT — PRZYJACIEL BD (Faktura BD + 12% Zysku) (zł)",
		"TORT — INWESTORZY (10% Zysku) (zł)",
	}

	simYearLabels = [3]string{
		"SUMA ROK 1 (M01-M12)",
		"SUMA ROK 2 (M13-M24)",
		"SUMA ROK 3 (M25-M36)",
	}
)

func csvCell(s string) string {
	if strings.ContainsAny(s, csvQuoteChars) {
		return csvDoubleQuote + strings.ReplaceAll(s, csvDoubleQuote, csvEscapedQuote) + csvDoubleQuote
	}
	return s
}

func writeCSVRow(sb *strings.Builder, row calcCSVRowTemplate) {
	sb.WriteString(csvCell(row.c1))
	sb.WriteByte(',')
	sb.WriteString(csvCell(row.c2))
	sb.WriteByte(',')
	sb.WriteString(csvCell(row.c3))
	sb.WriteByte(',')
	sb.WriteString(csvCell(row.c4))
	sb.WriteByte(',')
	sb.WriteString(csvCell(row.c5))
	sb.WriteByte('\n')
}

func writeCalculatedFinanceCSV(
	rootDir string,
	cfg financeConfig,
	years [3]deckYearCalc,
) {
	docsDir := filepath.Join(rootDir, dirDocs)
	if info, err := os.Stat(docsDir); err != nil || !info.IsDir() {
		return
	}

	y1, y2, y3 := years[0], years[1], years[2]
	archTotal3Y := y1.archTotalYearly + y2.archTotalYearly + y3.archTotalYearly
	bdTotal3Y := y1.bdTotalCashYearly + y2.bdTotalCashYearly + y3.bdTotalCashYearly
	invDividend3Y := y1.invDivYearly + y2.invDivYearly + y3.invDivYearly

	valMin := y3.revTotal * cfg.multMin
	valMax := y3.revTotal * cfg.multMax
	stakeMin := valMin * (cfg.invEquityPct / 100.0)
	stakeMax := valMax * (cfg.invEquityPct / 100.0)
	archStakeMin := valMin * (cfg.archEquityPct / 100.0)
	archStakeMax := valMax * (cfg.archEquityPct / 100.0)
	bdStakeMin := valMin * (cfg.bdEquityPct / 100.0)
	bdStakeMax := valMax * (cfg.bdEquityPct / 100.0)
	roiMin := (invDividend3Y + stakeMin) / cfg.srcInvestors
	roiMax := (invDividend3Y + stakeMax) / cfg.srcInvestors

	totalBudget := cfg.srcGrants + cfg.srcInvestors + cfg.srcCrowdfunding + cfg.srcOwnSales
	nonDilutive := cfg.srcGrants + cfg.srcCrowdfunding + cfg.srcOwnSales

	var sb strings.Builder
	writeCSVRow(&sb, calcCSVHeaderRow)
	writeCSVRow(&sb, calcSection0Header)

	writeCSVRow(&sb, calcCSVRowTemplate{
		c1: fmt.Sprintf(fmtRow0ALabel, int(cfg.archEquityPct)),
		c2: fmt.Sprintf(fmtCashYearMonth, fmtPolishInt(int64(y1.archTotalYearly)), fmtPolishInt(int64(y1.archTotalMonthly))),
		c3: fmt.Sprintf(fmtCashYearMonth, fmtPolishInt(int64(y2.archTotalYearly)), fmtPolishInt(int64(y2.archTotalMonthly))),
		c4: fmt.Sprintf(fmtCashYearMonth, fmtPolishInt(int64(y3.archTotalYearly)), fmtPolishInt(int64(y3.archTotalMonthly))),
		c5: fmt.Sprintf(fmtRow0ADetail, fmtPolishInt(int64(archTotal3Y)), int(cfg.archEquityPct), fmtPolishDec(archStakeMin/1000000.0, 2), fmtPolishDec(archStakeMax/1000000.0, 2)),
	})
	writeCSVRow(&sb, calcCSVRowTemplate{
		c1: fmt.Sprintf(fmtRow0BLabel, int(cfg.bdEquityPct)),
		c2: fmt.Sprintf(fmtCashYearMonth, fmtPolishInt(int64(y1.bdTotalCashYearly)), fmtPolishInt(int64(y1.bdTotalCashMonthly))),
		c3: fmt.Sprintf(fmtCashYearMonth, fmtPolishInt(int64(y2.bdTotalCashYearly)), fmtPolishInt(int64(y2.bdTotalCashMonthly))),
		c4: fmt.Sprintf(fmtCashYearMonth, fmtPolishInt(int64(y3.bdTotalCashYearly)), fmtPolishInt(int64(y3.bdTotalCashMonthly))),
		c5: fmt.Sprintf(fmtRow0BDetail, fmtPolishInt(int64(bdTotal3Y)), int(cfg.bdEquityPct), fmtPolishDec(bdStakeMin/1000000.0, 2), fmtPolishDec(bdStakeMax/1000000.0, 2)),
	})
	writeCSVRow(&sb, calcCSVRowTemplate{
		c1: fmt.Sprintf(fmtRow0CLabel, int(cfg.invEquityPct), fmtPolishInt(int64(cfg.srcInvestors))),
		c2: fmt.Sprintf(fmtDividendPLN, fmtPolishInt(int64(y1.invDivYearly))),
		c3: fmt.Sprintf(fmtDividendPLN, fmtPolishInt(int64(y2.invDivYearly))),
		c4: fmt.Sprintf(fmtDividendPLN, fmtPolishInt(int64(y3.invDivYearly))),
		c5: fmt.Sprintf(fmtRow0CDetail, fmtPolishInt(int64(invDividend3Y)), int(cfg.invEquityPct), fmtPolishDec(stakeMin/1000000.0, 2), fmtPolishDec(stakeMax/1000000.0, 2), fmtPolishDec(roiMin, 1), fmtPolishDec(roiMax, 1)),
	})

	sec1Hdr := calcSection1Header
	sec1Hdr.c2 = fmt.Sprintf(fmtCurrencyPLN, fmtPolishInt(int64(totalBudget)))
	sec1Hdr.c4 = fmt.Sprintf(fmtRow1NonDil, fmtPolishInt(int64(nonDilutive)))
	writeCSVRow(&sb, sec1Hdr)

	for _, spec := range calcSection1Rows {
		col4 := spec.col4
		if spec.teamShare {
			col4 = fmt.Sprintf(fmtRow1BShares, int(cfg.invEquityPct), int(cfg.archEquityPct+cfg.bdEquityPct))
		}
		writeCSVRow(&sb, calcCSVRowTemplate{
			c1: spec.label,
			c2: fmt.Sprintf(fmtCurrencyPLN, fmtPolishInt(int64(spec.amount(cfg)))),
			c3: spec.pctCol,
			c4: col4,
			c5: spec.detail,
		})
	}

	sec2Hdr := calcSection2Header
	sec2Hdr.c2 = fmt.Sprintf(fmtCurrencyPLN, fmtPolishInt(int64(totalBudget)))
	writeCSVRow(&sb, sec2Hdr)

	for _, spec := range calcSection2Rows {
		writeCSVRow(&sb, calcCSVRowTemplate{
			c1: spec.label,
			c2: fmt.Sprintf(fmtCurrencyPLN, fmtPolishInt(int64(spec.amount(cfg)))),
			c3: spec.pctCol,
			c4: spec.col4,
			c5: spec.detail,
		})
	}

	writeCSVRow(&sb, calcSection3Header)
	for _, spec := range calcSection3Rows {
		v1 := spec.val(y1)
		v2 := spec.val(y2)
		v3 := spec.val(y3)
		detail := spec.detail
		if spec.sum3Y {
			detail = fmt.Sprintf(fmtTotal3YPLN, fmtPolishInt(int64(v1+v2+v3)))
		} else if spec.netDesc {
			detail = fmt.Sprintf(fmtRow3NetSplit, fmtPolishInt(int64(v1+v2+v3)), int(cfg.archEquityPct), int(cfg.bdEquityPct), int(cfg.invEquityPct))
		}
		writeCSVRow(&sb, calcCSVRowTemplate{
			c1: spec.label,
			c2: fmt.Sprintf(fmtCurrencyPLN, fmtPolishInt(int64(v1))),
			c3: fmt.Sprintf(fmtCurrencyPLN, fmtPolishInt(int64(v2))),
			c4: fmt.Sprintf(fmtCurrencyPLN, fmtPolishInt(int64(v3))),
			c5: detail,
		})
	}

	if err := os.WriteFile(filepath.Join(docsDir, financeCalcCSVFile), []byte(sb.String()), 0644); err != nil {
		return
	}
}

func write36MonthSimulationCSV(
	rootDir string,
	allMonths []deckMonthRow,
	years [3]deckYearCalc,
) {
	docsDir := filepath.Join(rootDir, dirDocs)
	if info, err := os.Stat(docsDir); err != nil || !info.IsDir() {
		return
	}
	var sb strings.Builder
	writeMultiCSVRow := func(cols ...string) {
		for i, c := range cols {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(csvCell(c))
		}
		sb.WriteByte('\n')
	}

	writeMultiCSVRow(simCSVHeaders[:]...)

	for _, r := range allMonths {
		writeMultiCSVRow(
			r.Code,
			r.Milestone,
			fmtPolishInt(r.OrigUnits),
			fmtPolishInt(r.RevOrig),
			fmtPolishInt(r.RevLic),
			fmtPolishInt(r.RevCat),
			fmtPolishInt(r.RevTotal),
			fmtPolishInt(r.RDArch),
			fmtPolishInt(r.BDCost),
			fmtPolishInt(r.Office),
			fmtPolishInt(r.EBITDA),
			fmtPolishInt(r.NetProfit),
			fmtPolishInt(r.ArchTotal),
			fmtPolishInt(r.BDTotal),
			fmtPolishInt(r.InvDiv),
		)
	}

	for yr := 0; yr < 3; yr++ {
		yc := years[yr]
		writeMultiCSVRow(
			simYearLabels[yr],
			simYearSummaryDesc,
			fmtPolishInt(int64(yc.origUnits)),
			fmtPolishInt(int64(yc.revOrig)),
			fmtPolishInt(int64(yc.revLic)),
			fmtPolishInt(int64(yc.revCat)),
			fmtPolishInt(int64(yc.revTotal)),
			fmtPolishInt(int64(yc.rdArchYearly)),
			fmtPolishInt(int64(yc.bdTotalYearly)),
			fmtPolishInt(int64(yc.officeYearly)),
			fmtPolishInt(int64(yc.ebitdaCompany)),
			fmtPolishInt(int64(yc.netProfit)),
			fmtPolishInt(int64(yc.archTotalYearly)),
			fmtPolishInt(int64(yc.bdTotalCashYearly)),
			fmtPolishInt(int64(yc.invDivYearly)),
		)
	}

	y1, y2, y3 := years[0], years[1], years[2]
	writeMultiCSVRow(
		simTotal3YLabel,
		simTotal3YDesc,
		fmtPolishInt(int64(y1.origUnits+y2.origUnits+y3.origUnits)),
		fmtPolishInt(int64(y1.revOrig+y2.revOrig+y3.revOrig)),
		fmtPolishInt(int64(y1.revLic+y2.revLic+y3.revLic)),
		fmtPolishInt(int64(y1.revCat+y2.revCat+y3.revCat)),
		fmtPolishInt(int64(y1.revTotal+y2.revTotal+y3.revTotal)),
		fmtPolishInt(int64(y1.rdArchYearly+y2.rdArchYearly+y3.rdArchYearly)),
		fmtPolishInt(int64(y1.bdTotalYearly+y2.bdTotalYearly+y3.bdTotalYearly)),
		fmtPolishInt(int64(y1.officeYearly+y2.officeYearly+y3.officeYearly)),
		fmtPolishInt(int64(y1.ebitdaCompany+y2.ebitdaCompany+y3.ebitdaCompany)),
		fmtPolishInt(int64(y1.netProfit+y2.netProfit+y3.netProfit)),
		fmtPolishInt(int64(y1.archTotalYearly+y2.archTotalYearly+y3.archTotalYearly)),
		fmtPolishInt(int64(y1.bdTotalCashYearly+y2.bdTotalCashYearly+y3.bdTotalCashYearly)),
		fmtPolishInt(int64(y1.invDivYearly+y2.invDivYearly+y3.invDivYearly)),
	)

	if err := os.WriteFile(filepath.Join(docsDir, financeSimCSVFile), []byte(sb.String()), 0644); err != nil {
		return
	}
}
