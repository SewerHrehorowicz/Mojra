package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mojra/internal/tpl"
)

type deckYearCalc struct {
	origUnits, catUnits                           float64
	revUni, revStudio, revLic, revOrig, revCat    float64
	revTotal, varCosts, grossMargin               float64
	rdArchYearly, bdRetYearly, bdCommYearly       float64
	bdTotalYearly, officeYearly, bdAndAdminYearly float64
	opexTotal, ebitdaCompany, citTax, netProfit   float64
	archDivYearly, bdDivYearly, invDivYearly      float64
	archTotalYearly, archTotalMonthly             float64
	bdTotalCashYearly, bdTotalCashMonthly         float64
}

type deckMonthRow struct {
	MonthNum  int
	Code      string
	Milestone string
	OrigUnits int64
	RevOrig   int64
	RevLic    int64
	RevCat    int64
	RevTotal  int64
	RDArch    int64
	BDCost    int64
	Office    int64
	EBITDA    int64
	NetProfit int64
	ArchTotal int64
	BDTotal   int64
	InvDiv    int64
}

// LoadDeckFinanceContext reads docs/finanse.cfg, evaluates all bottom-up financial formulas
// for both the Investor Deck (P&L, OPEX R&D/BD, EBITDA, ROI) and the Founder's Master Overview
// (36-month simulation M1-M36, stakeholder pie split between Lead Architect 78%, Business Co-Founder 12%,
// and Investors 10%, plus 3 realism scenarios), writes docs/kalkulator-finansowy.csv and docs/symulacja-36-miesiecy.csv,
// and returns a *tpl.RenderContext.
func LoadDeckFinanceContext(rootDir string) *tpl.RenderContext {
	var cfg financeConfig
	for _, fd := range financeConfigSchema {
		*fd.ptr(&cfg) = fd.defaultVal
	}

	cfgPath := filepath.Join(rootDir, dirDocs, financeCfgFile)
	if raw, err := os.ReadFile(cfgPath); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			before, _, _ := strings.Cut(line, "#")
			clean := strings.TrimSpace(strings.ReplaceAll(before, " ", ""))
			k, vStr, ok := strings.Cut(clean, "=")
			if !ok || k == "" || vStr == "" {
				continue
			}
			if v, pErr := strconv.ParseFloat(vStr, 64); pErr == nil {
				for _, fd := range financeConfigSchema {
					if fd.key == k {
						*fd.ptr(&cfg) = v
						break
					}
				}
			}
		}
	}

	if cfg.archEquityPct <= 0 || math.Abs((cfg.archEquityPct+cfg.bdEquityPct+cfg.invEquityPct)-100.0) > 0.01 {
		cfg.archEquityPct = 100.0 - cfg.bdEquityPct - cfg.invEquityPct
	}
	teamEquityPct := cfg.archEquityPct + cfg.bdEquityPct

	officeYearly := cfg.officeMonthly * 12.0
	authorPct := 100.0 - cfg.catFeePct
	bookFee := cfg.bookPrice * (cfg.catFeePct / 100.0)
	authorShare := cfg.bookPrice - bookFee

	totalBudget := cfg.srcGrants + cfg.srcInvestors + cfg.srcCrowdfunding + cfg.srcOwnSales
	nonDilutive := cfg.srcGrants + cfg.srcCrowdfunding + cfg.srcOwnSales
	nonDilutivePct := int(math.Round((nonDilutive / totalBudget) * 100.0))

	minTicketPct := cfg.invEquityPct * (cfg.minTicket / cfg.srcInvestors)

	budPremiery := cfg.costActors + cfg.costSoundStudio + cfg.costAuthors + cfg.costIllustSFX
	budIT := cfg.costAuditCert + cfg.costMobileDev + cfg.costShopInfra + cfg.costLegal
	budRunway := cfg.costRunway
	budRozwojTotal := budIT + budRunway
	budSprzedaz := cfg.costCampaignCF + cfg.costPromoPremieres + cfg.costWorkshopsB2B + cfg.costFairsContests

	var years [3]deckYearCalc
	for yr := 0; yr < 3; yr++ {
		in := cfg.years[yr]
		var r deckYearCalc
		r.origUnits = in.origTitles * in.origUnitsPerT
		r.catUnits = in.catTitles * in.catUnitsPerT
		r.revUni = in.universities * cfg.uniPrice
		r.revStudio = in.indieLicenses*cfg.indiePrice + in.proLicenses*cfg.proPrice
		r.revLic = r.revUni + r.revStudio
		r.revOrig = r.origUnits * cfg.origMargin
		r.revCat = r.catUnits * bookFee
		r.revTotal = r.revLic + r.revOrig + r.revCat

		r.varCosts = math.Round((r.revLic + r.revCat) * (cfg.varCostPct / 100.0))
		r.grossMargin = r.revTotal - r.varCosts

		r.rdArchYearly = math.Round(in.rdArchMonthly * 12.0)
		r.bdRetYearly = math.Round(in.bdRetMonthly * 12.0)
		r.bdCommYearly = math.Round(r.revLic * (cfg.bdCommPct / 100.0))
		r.bdTotalYearly = r.bdRetYearly + r.bdCommYearly
		r.officeYearly = math.Round(officeYearly)
		r.bdAndAdminYearly = r.bdTotalYearly + r.officeYearly
		r.opexTotal = r.rdArchYearly + r.bdAndAdminYearly

		r.ebitdaCompany = math.Max(0, r.grossMargin-r.opexTotal)
		r.citTax = math.Round(r.ebitdaCompany * (cfg.citPct / 100.0))
		r.netProfit = r.ebitdaCompany - r.citTax

		r.archDivYearly = math.Round(r.netProfit * (cfg.archEquityPct / 100.0))
		r.bdDivYearly = math.Round(r.netProfit * (cfg.bdEquityPct / 100.0))
		r.invDivYearly = r.netProfit - r.archDivYearly - r.bdDivYearly

		r.archTotalYearly = r.rdArchYearly + r.archDivYearly
		r.archTotalMonthly = math.Round(r.archTotalYearly / 12.0)

		r.bdTotalCashYearly = r.bdTotalYearly + r.bdDivYearly
		r.bdTotalCashMonthly = math.Round(r.bdTotalCashYearly / 12.0)
		years[yr] = r
	}

	y1, y2, y3 := years[0], years[1], years[2]

	rev3Y := y1.revTotal + y2.revTotal + y3.revTotal
	ebitda3Y := y1.ebitdaCompany + y2.ebitdaCompany + y3.ebitdaCompany
	netProfit3Y := y1.netProfit + y2.netProfit + y3.netProfit

	rdArch3Y := y1.rdArchYearly + y2.rdArchYearly + y3.rdArchYearly
	archDiv3Y := y1.archDivYearly + y2.archDivYearly + y3.archDivYearly
	archTotal3Y := y1.archTotalYearly + y2.archTotalYearly + y3.archTotalYearly

	bdOps3Y := y1.bdTotalYearly + y2.bdTotalYearly + y3.bdTotalYearly
	bdDiv3Y := y1.bdDivYearly + y2.bdDivYearly + y3.bdDivYearly
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

	bepMonthlyOPEX := cfg.years[0].rdArchMonthly + cfg.years[0].bdRetMonthly + cfg.officeMonthly
	bepMonthlyUnits := int64(math.Ceil(bepMonthlyOPEX / cfg.origMargin))
	bepDailyUnits := int64(math.Ceil(float64(bepMonthlyUnits) / 30.0))

	pctBudPremiery := int(math.Round((budPremiery / totalBudget) * 100.0))
	pctBudRozwoj := int(math.Round((budRozwojTotal / totalBudget) * 100.0))
	pctBudSprzedaz := 100 - pctBudPremiery - pctBudRozwoj

	monthsByYear, allMonths := build36MonthSimulation(cfg, years)

	writeCalculatedFinanceCSV(rootDir, cfg, years)
	write36MonthSimulationCSV(rootDir, allMonths, years)

	vars := map[string]any{
		ctxKeyArchEquityPct:    fmtPolishInt(int64(cfg.archEquityPct)),
		ctxKeyBDEquityPct:      fmtPolishInt(int64(cfg.bdEquityPct)),
		ctxKeyTeamEquityPct:    fmtPolishInt(int64(teamEquityPct)),
		ctxKeyInvEquityPct:     fmtPolishInt(int64(cfg.invEquityPct)),
		ctxKeyFounderEquityPct: fmtPolishInt(int64(teamEquityPct)),

		ctxKeyArchHourlyRate: fmtPolishInt(int64(cfg.archHourlyRate)),
		ctxKeyRDArch3Y:       fmtPolishInt(int64(rdArch3Y)),

		ctxKeyBDCommPct: fmtPolishInt(int64(cfg.bdCommPct)),
		ctxKeyBDOps3Y:   fmtPolishInt(int64(bdOps3Y)),

		ctxKeyArchDiv3Y:    fmtPolishInt(int64(archDiv3Y)),
		ctxKeyArchSum3Y:    fmtPolishInt(int64(archTotal3Y)),
		ctxKeyArchValR3Mln: fmt.Sprintf(fmtValRangeMln, fmtPolishDec(archStakeMin/1000000.0, 2), fmtPolishDec(archStakeMax/1000000.0, 2)),

		ctxKeyBDDiv3Y:    fmtPolishInt(int64(bdDiv3Y)),
		ctxKeyBDSum3Y:    fmtPolishInt(int64(bdTotal3Y)),
		ctxKeyBDValR3Mln: fmt.Sprintf(fmtValRangeMln, fmtPolishDec(bdStakeMin/1000000.0, 2), fmtPolishDec(bdStakeMax/1000000.0, 2)),

		ctxKeyBEPMonthOPEX:  fmtPolishInt(int64(bepMonthlyOPEX)),
		ctxKeyBEPMonthUnits: fmtPolishInt(bepMonthlyUnits),
		ctxKeyBEPDayUnits:   fmtPolishInt(bepDailyUnits),

		ctxKeyFounderNetMonth: fmtPolishInt(int64(cfg.founderNetMonthly)),
		ctxKeyFounderNetYear:  fmtPolishInt(int64(cfg.founderNetMonthly * 12)),
		ctxKeyFounderSum3Y:    fmtPolishInt(int64(archTotal3Y)),

		ctxKeyOrigPrice:   fmtPolishInt(int64(cfg.origPrice)),
		ctxKeyOrigMargin:  fmtPolishInt(int64(cfg.origMargin)),
		ctxKeyBookPrice:   fmtPolishInt(int64(cfg.bookPrice)),
		ctxKeyCatFeePct:   fmtPolishInt(int64(cfg.catFeePct)),
		ctxKeyAuthorPct:   fmtPolishInt(int64(authorPct)),
		ctxKeyAuthorShare: fmtPolishInt(int64(authorShare)),
		ctxKeyBookFeeUnit: fmtPolishInt(int64(bookFee)),
		ctxKeyIndiePrice:  fmtPolishInt(int64(cfg.indiePrice)),
		ctxKeyProPrice:    fmtPolishInt(int64(cfg.proPrice)),
		ctxKeyUniPrice:    fmtPolishInt(int64(cfg.uniPrice)),
		ctxKeyOfficeMonth: fmtPolishInt(int64(cfg.officeMonthly)),

		ctxKeyTotalBudget:      fmtPolishInt(int64(totalBudget)),
		ctxKeyTotalBudgetK:     fmtPolishInt(int64(totalBudget / 1000.0)),
		ctxKeySrcGrants:        fmtPolishInt(int64(cfg.srcGrants)),
		ctxKeySrcGrantsK:       fmtPolishInt(int64(cfg.srcGrants / 1000.0)),
		ctxKeySrcInvestors:     fmtPolishInt(int64(cfg.srcInvestors)),
		ctxKeySrcInvestorsK:    fmtPolishInt(int64(cfg.srcInvestors / 1000.0)),
		ctxKeySrcCrowdfunding:  fmtPolishInt(int64(cfg.srcCrowdfunding)),
		ctxKeySrcCrowdfundingK: fmtPolishInt(int64(cfg.srcCrowdfunding / 1000.0)),
		ctxKeySrcOwn:           fmtPolishInt(int64(cfg.srcOwnSales)),
		ctxKeySrcOwnK:          fmtPolishInt(int64(cfg.srcOwnSales / 1000.0)),
		ctxKeyNonDilutive:      fmtPolishInt(int64(nonDilutive)),
		ctxKeyNonDilutiveK:     fmtPolishInt(int64(nonDilutive / 1000.0)),
		ctxKeyNonDilutivePct:   strconv.Itoa(nonDilutivePct),

		ctxKeyInvestAmount:  fmtPolishInt(int64(cfg.srcInvestors)),
		ctxKeyInvestAmountK: fmtPolishInt(int64(cfg.srcInvestors / 1000.0)),
		ctxKeyMinTicket:     fmtPolishInt(int64(cfg.minTicket)),
		ctxKeyMinTicketPct:  fmtPolishDec(minTicketPct, 2),

		ctxKeyBudPremieres:    fmtPolishInt(int64(budPremiery)),
		ctxKeyBudPremieresK:   fmtPolishInt(int64(budPremiery / 1000.0)),
		ctxKeyBudIT:           fmtPolishInt(int64(budIT)),
		ctxKeyBudITK:          fmtPolishInt(int64(budIT / 1000.0)),
		ctxKeyBudRunway:       fmtPolishInt(int64(budRunway)),
		ctxKeyBudDev:          fmtPolishInt(int64(budRozwojTotal)),
		ctxKeyBudDevK:         fmtPolishInt(int64(budRozwojTotal / 1000.0)),
		ctxKeyBudSales:        fmtPolishInt(int64(budSprzedaz)),
		ctxKeyBudSalesK:       fmtPolishInt(int64(budSprzedaz / 1000.0)),
		ctxKeyBudPremieresPct: strconv.Itoa(pctBudPremiery),
		ctxKeyBudDevPct:       strconv.Itoa(pctBudRozwoj),
		ctxKeyBudSalesPct:     strconv.Itoa(pctBudSprzedaz),

		ctxKeyCostActors:        fmtPolishInt(int64(cfg.costActors)),
		ctxKeyCostSoundStudio:   fmtPolishInt(int64(cfg.costSoundStudio)),
		ctxKeyCostAuthors:       fmtPolishInt(int64(cfg.costAuthors)),
		ctxKeyCostIllustSFX:     fmtPolishInt(int64(cfg.costIllustSFX)),
		ctxKeyCostAuditCert:     fmtPolishInt(int64(cfg.costAuditCert)),
		ctxKeyCostMobileDev:     fmtPolishInt(int64(cfg.costMobileDev)),
		ctxKeyCostShopInfra:     fmtPolishInt(int64(cfg.costShopInfra)),
		ctxKeyCostLegal:         fmtPolishInt(int64(cfg.costLegal)),
		ctxKeyCostCampaignCF:    fmtPolishInt(int64(cfg.costCampaignCF)),
		ctxKeyCostPromoPrem:     fmtPolishInt(int64(cfg.costPromoPremieres)),
		ctxKeyCostWorkshopsB2B:  fmtPolishInt(int64(cfg.costWorkshopsB2B)),
		ctxKeyCostFairsContests: fmtPolishInt(int64(cfg.costFairsContests)),
		ctxKeyCostActorsK:       fmtPolishInt(int64(cfg.costActors / 1000.0)),
		ctxKeyCostSoundStudioK:  fmtPolishInt(int64(cfg.costSoundStudio / 1000.0)),
		ctxKeyCostAuthorsK:      fmtPolishInt(int64(cfg.costAuthors / 1000.0)),
		ctxKeyCostIllustSFXK:    fmtPolishInt(int64(cfg.costIllustSFX / 1000.0)),
		ctxKeyCostMobileDevK:    fmtPolishInt(int64(cfg.costMobileDev / 1000.0)),
		ctxKeyCostStudioDevK:    fmtPolishInt(int64(cfg.costAuditCert / 1000.0)),

		ctxKeyRev3Y:       fmtPolishInt(int64(rev3Y)),
		ctxKeyProfitR1K:   fmtPolishInt(int64(math.Round(y1.ebitdaCompany / 1000.0))),
		ctxKeyProfitR3Mln: fmtPolishDec(y3.ebitdaCompany/1000000.0, 2),
		ctxKeyEBITDA3Y:    fmtPolishInt(int64(ebitda3Y)),
		ctxKeyNetProfit3Y: fmtPolishInt(int64(netProfit3Y)),

		ctxKeyInvProfit3Y:     fmtPolishInt(int64(invDividend3Y)),
		ctxKeyCompanyValR3Mln: fmt.Sprintf(fmtValRangeMln, fmtPolishDec(valMin/1000000.0, 2), fmtPolishDec(valMax/1000000.0, 2)),
		ctxKeyInvStakeValR3:   fmt.Sprintf(fmtValRangeMln, fmtPolishDec(stakeMin/1000000.0, 2), fmtPolishDec(stakeMax/1000000.0, 2)),
		ctxKeyInvROIX:         fmt.Sprintf(fmtROIRangeX, fmtPolishDec(roiMin, 1), fmtPolishDec(roiMax, 1)),
	}

	for yr := 0; yr < 3; yr++ {
		yk := deckYearVarKeys[yr]
		in := cfg.years[yr]
		yc := years[yr]
		vars[yk.rdArchMonthly] = fmtPolishInt(int64(in.rdArchMonthly))
		vars[yk.rdArchYearly] = fmtPolishInt(int64(yc.rdArchYearly))
		vars[yk.bdRetMonthly] = fmtPolishInt(int64(in.bdRetMonthly))
		vars[yk.bdOpsYearly] = fmtPolishInt(int64(yc.bdTotalYearly))
		vars[yk.bdAdminYearly] = fmtPolishInt(int64(yc.bdAndAdminYearly))
		vars[yk.archDivYearly] = fmtPolishInt(int64(yc.archDivYearly))
		vars[yk.archTotalYearly] = fmtPolishInt(int64(yc.archTotalYearly))
		vars[yk.archTotalMonth] = fmtPolishInt(int64(yc.archTotalMonthly))
		vars[yk.bdDivYearly] = fmtPolishInt(int64(yc.bdDivYearly))
		vars[yk.bdTotalYearly] = fmtPolishInt(int64(yc.bdTotalCashYearly))
		vars[yk.bdTotalMonth] = fmtPolishInt(int64(yc.bdTotalCashMonthly))
		vars[yk.invDivYearly] = fmtPolishInt(int64(yc.invDivYearly))
		vars[yk.founderNetMonth] = fmtPolishInt(int64(yc.archTotalMonthly))
		vars[yk.universities] = fmtPolishInt(int64(in.universities))
		vars[yk.indie] = fmtPolishInt(int64(in.indieLicenses))
		vars[yk.pro] = fmtPolishInt(int64(in.proLicenses))
		vars[yk.origTitles] = fmtPolishInt(int64(in.origTitles))
		vars[yk.origUnitsPerT] = fmtPolishInt(int64(in.origUnitsPerT))
		vars[yk.origTotalUnits] = fmtPolishInt(int64(yc.origUnits))
		vars[yk.catTitles] = fmtPolishInt(int64(in.catTitles))
		vars[yk.catTotalUnits] = fmtPolishInt(int64(yc.catUnits))
		vars[yk.revOrig] = fmtPolishInt(int64(yc.revOrig))
		vars[yk.revCat] = fmtPolishInt(int64(yc.revCat))
		vars[yk.revLic] = fmtPolishInt(int64(yc.revLic))
		vars[yk.revTotal] = fmtPolishInt(int64(yc.revTotal))
		vars[yk.ebitda] = fmtPolishInt(int64(yc.ebitdaCompany))
		vars[yk.netProfit] = fmtPolishInt(int64(yc.netProfit))
		vars[yk.months] = monthsByYear[yr]
	}

	return tpl.NewContext(vars)
}

func distribute12Months(total float64, weights [12]float64) [12]int64 {
	var out [12]int64
	target := int64(math.Round(total))
	if target <= 0 {
		return out
	}
	var sumW float64
	for _, w := range weights {
		sumW += w
	}
	var running int64
	for i := 0; i < 11; i++ {
		v := int64(math.Round(total * (weights[i] / sumW)))
		out[i] = v
		running += v
	}
	out[11] = target - running
	return out
}

func build36MonthSimulation(
	cfg financeConfig,
	years [3]deckYearCalc,
) ([3][]map[string]any, []deckMonthRow) {
	wOrigY1 := [12]float64{0, 450, 550, 700, 1050, 1050, 1150, 1100, 1250, 1200, 1350, 1350}
	wLicY1 := [12]float64{1, 2, 16, 4, 16, 4, 3, 4, 18, 16, 8, 8}
	wCatY1 := [12]float64{0, 1, 2, 3, 6, 8, 9, 10, 12, 14, 17, 18}
	wRDY1 := [12]float64{12, 12, 12, 12, 24, 24, 24, 24, 24, 24, 24, 24}
	wBDRetY1 := [12]float64{1.5, 1.5, 1.5, 1.5, 3.75, 3.75, 3.75, 3.75, 3.75, 3.75, 3.75, 3.75}

	wOrigY2 := [12]float64{1350, 1550, 1500, 1550, 1750, 1700, 1750, 1850, 1800, 1950, 2000, 2050}
	wLicY2 := [12]float64{8, 7, 14, 6, 6, 5, 5, 6, 16, 12, 8, 7}
	wCatY2 := [12]float64{5, 6, 6, 7, 7, 8, 8, 9, 10, 10, 12, 12}
	wUniform := [12]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}

	wOrigY3 := [12]float64{2300, 2600, 2550, 2800, 2750, 2900, 3150, 3050, 3100, 3450, 3600, 3750}
	wLicY3 := [12]float64{8, 7, 13, 6, 6, 5, 5, 6, 16, 12, 8, 8}
	wCatY3 := [12]float64{6, 6, 7, 7, 8, 8, 8, 9, 9, 10, 11, 11}

	buildYearMonths := func(
		yrIdx int,
		yc deckYearCalc,
		wOrig, wLic, wCat, wRD, wBDRet [12]float64,
	) ([]map[string]any, []deckMonthRow) {
		origArr := distribute12Months(yc.origUnits, wOrig)
		revOrigArr := distribute12Months(yc.revOrig, wOrig)
		revLicArr := distribute12Months(yc.revLic, wLic)
		revCatArr := distribute12Months(yc.revCat, wCat)
		varCostArr := distribute12Months(yc.varCosts, wLic)
		rdArr := distribute12Months(yc.rdArchYearly, wRD)
		bdRetArr := distribute12Months(yc.bdRetYearly, wBDRet)
		bdCommArr := distribute12Months(yc.bdCommYearly, wLic)
		offArr := distribute12Months(yc.officeYearly, wUniform)

		var rows []deckMonthRow
		var maps []map[string]any
		var sumEBITDA, sumNet, sumArchDiv, sumBDDiv int64

		for i := 0; i < 12; i++ {
			mNum := yrIdx*12 + i + 1
			revTot := revOrigArr[i] + revLicArr[i] + revCatArr[i]
			grossM := revTot - varCostArr[i]
			bdCost := bdRetArr[i] + bdCommArr[i]
			opex := rdArr[i] + bdCost + offArr[i]
			ebitda := grossM - opex
			if i == 11 {
				ebitda = int64(math.Round(yc.ebitdaCompany)) - sumEBITDA
			}
			sumEBITDA += ebitda

			var netP int64
			if i == 11 {
				netP = int64(math.Round(yc.netProfit)) - sumNet
			} else {
				if ebitda > 0 {
					netP = int64(math.Round(float64(ebitda) * (1.0 - cfg.citPct/100.0)))
				} else {
					netP = ebitda
				}
			}
			sumNet += netP

			var archDiv, bdDiv, invDiv int64
			if i == 11 {
				archDiv = int64(math.Round(yc.archDivYearly)) - sumArchDiv
				bdDiv = int64(math.Round(yc.bdDivYearly)) - sumBDDiv
				invDiv = netP - archDiv - bdDiv
			} else if netP > 0 {
				archDiv = int64(math.Round(float64(netP) * (cfg.archEquityPct / 100.0)))
				bdDiv = int64(math.Round(float64(netP) * (cfg.bdEquityPct / 100.0)))
				invDiv = netP - archDiv - bdDiv
			}
			sumArchDiv += archDiv
			sumBDDiv += bdDiv

			archTot := rdArr[i] + archDiv
			bdTot := bdCost + bdDiv

			code := fmt.Sprintf(fmtMonthCode, mNum)
			row := deckMonthRow{
				MonthNum:  mNum,
				Code:      code,
				Milestone: deckMilestones[mNum-1],
				OrigUnits: origArr[i],
				RevOrig:   revOrigArr[i],
				RevLic:    revLicArr[i],
				RevCat:    revCatArr[i],
				RevTotal:  revTot,
				RDArch:    rdArr[i],
				BDCost:    bdCost,
				Office:    offArr[i],
				EBITDA:    ebitda,
				NetProfit: netP,
				ArchTotal: archTot,
				BDTotal:   bdTot,
				InvDiv:    invDiv,
			}
			rows = append(rows, row)
			maps = append(maps, map[string]any{
				monthKeyCode:      code,
				monthKeyMilestone: row.Milestone,
				monthKeyOrigUnits: fmtPolishInt(row.OrigUnits),
				monthKeyRevOrig:   fmtPolishInt(row.RevOrig),
				monthKeyRevLic:    fmtPolishInt(row.RevLic),
				monthKeyRevCat:    fmtPolishInt(row.RevCat),
				monthKeyRevTotal:  fmtPolishInt(row.RevTotal),
				monthKeyRDArch:    fmtPolishInt(row.RDArch),
				monthKeyBDCost:    fmtPolishInt(row.BDCost),
				monthKeyEBITDA:    fmtPolishInt(row.EBITDA),
				monthKeyNetProfit: fmtPolishInt(row.NetProfit),
				monthKeyArchTotal: fmtPolishInt(row.ArchTotal),
				monthKeyBDTotal:   fmtPolishInt(row.BDTotal),
				monthKeyInvDiv:    fmtPolishInt(row.InvDiv),
			})
		}
		return maps, rows
	}

	var monthsByYear [3][]map[string]any
	mY1, rY1 := buildYearMonths(0, years[0], wOrigY1, wLicY1, wCatY1, wRDY1, wBDRetY1)
	mY2, rY2 := buildYearMonths(1, years[1], wOrigY2, wLicY2, wCatY2, wUniform, wUniform)
	mY3, rY3 := buildYearMonths(2, years[2], wOrigY3, wLicY3, wCatY3, wUniform, wUniform)
	monthsByYear[0] = mY1
	monthsByYear[1] = mY2
	monthsByYear[2] = mY3

	var all []deckMonthRow
	all = append(all, rY1...)
	all = append(all, rY2...)
	all = append(all, rY3...)
	return monthsByYear, all
}

func fmtPolishInt(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var out strings.Builder
	if neg {
		out.WriteByte('-')
	}
	rem := len(s) % 3
	if rem > 0 {
		out.WriteString(s[:rem])
		if len(s) > rem {
			out.WriteByte(' ')
		}
	}
	for i := rem; i < len(s); i += 3 {
		if i > rem {
			out.WriteByte(' ')
		}
		out.WriteString(s[i : i+3])
	}
	return out.String()
}

func fmtPolishDec(v float64, decimals int) string {
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	return strings.ReplaceAll(s, ".", ",")
}
