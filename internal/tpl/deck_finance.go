//go:build !(js && wasm)

package tpl

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
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
// and returns a RenderContext.
func LoadDeckFinanceContext(rootDir string) *RenderContext {
	cfg := map[string]float64{
		"udzial_architekta_proc":  78,
		"udzial_biznes_proc":      12,
		"udzial_inwestorow_proc":  10,
		"stawka_architekta_godz_zl": 160,
		"rd_architekt_r1_mies_zl": 20000,
		"rd_architekt_r2_mies_zl": 25000,
		"rd_architekt_r3_mies_zl": 30000,
		"prowizja_bd_b2b_proc":    15,
		"bd_retainer_r1_mies_zl":  3000,
		"bd_retainer_r2_mies_zl":  6000,
		"bd_retainer_r3_mies_zl":  10000,

		"wyplata_zalozyciela_netto_mies_zl": 24000,
		"koszt_zalozyciela_brutto_mies_zl":  24000,
		"koszty_biuro_ksiegowosc_mies_zl":   2000,
		"podatek_cit_efektywny_proc":        15,

		"cena_originals_zl":      45,
		"marza_originals_szt_zl": 32,
		"cena_ksiazki_zl":        40,
		"prowizja_katalog_proc":  15,
		"cena_indie_zl":          249,
		"cena_pro_zl":            599,
		"cena_uczelnia_zl":       7500,
		"koszt_zmienny_proc":     6,

		"zrodlo_dotacje_zl":         90000,
		"zrodlo_inwestorzy_zl":      80000,
		"zrodlo_crowdfunding_zl":    70000,
		"zrodlo_sprzedaz_wlasna_zl": 30000,
		"bilet_inwestora_min_zl":    40000,
		"mnoznik_wyceny_min":        4.5,
		"mnoznik_wyceny_max":        6.0,

		"koszt_aktorzy_zl":        60000,
		"koszt_studio_dzwiek_zl":  40000,
		"koszt_autorzy_zl":        34000,
		"koszt_ilustracje_sfx_zl": 16000,

		"koszt_audyt_cert_zl":   18000,
		"koszt_mobile_dev_zl":   24000,
		"koszt_sklep_infra_zl":  10000,
		"koszt_prawo_umowy_zl":  8000,
		"koszt_pas_startowy_zl": 0,

		"koszt_kampania_cf_zl":      18000,
		"koszt_promocja_premier_zl": 24000,
		"koszt_warsztaty_b2b_zl":    10000,
		"koszt_targi_konkursy_zl":   8000,

		"s_r1_uczelnie": 6, "s_r1_indie": 60, "s_r1_pro": 20,
		"s_r1_originals_tytuly": 4, "s_r1_originals_szt": 2800,
		"s_r1_katalog_tytuly": 4, "s_r1_katalog_szt": 800,

		"s_r2_uczelnie": 12, "s_r2_indie": 180, "s_r2_pro": 55,
		"s_r2_originals_tytuly": 8, "s_r2_originals_szt": 2600,
		"s_r2_katalog_tytuly": 15, "s_r2_katalog_szt": 1000,

		"s_r3_uczelnie": 22, "s_r3_indie": 400, "s_r3_pro": 110,
		"s_r3_originals_tytuly": 12, "s_r3_originals_szt": 3000,
		"s_r3_katalog_tytuly": 35, "s_r3_katalog_szt": 1200,
	}

	cfgPath := filepath.Join(rootDir, "docs", "finanse.cfg")
	if raw, err := os.ReadFile(cfgPath); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			eq := strings.IndexByte(line, '=')
			if eq < 0 {
				continue
			}
			k := strings.TrimSpace(line[:eq])
			vStr := strings.TrimSpace(line[eq+1:])
			if idx := strings.IndexByte(vStr, '#'); idx >= 0 {
				vStr = strings.TrimSpace(vStr[:idx])
			}
			if v, pErr := strconv.ParseFloat(strings.ReplaceAll(vStr, " ", ""), 64); pErr == nil {
				cfg[k] = v
			}
		}
	}

	invEquityPct := cfg["udzial_inwestorow_proc"]
	bdEquityPct := cfg["udzial_biznes_proc"]
	archEquityPct := cfg["udzial_architekta_proc"]
	if archEquityPct <= 0 || math.Abs((archEquityPct+bdEquityPct+invEquityPct)-100.0) > 0.01 {
		archEquityPct = 100.0 - bdEquityPct - invEquityPct
	}
	teamEquityPct := archEquityPct + bdEquityPct

	archHourly := cfg["stawka_architekta_godz_zl"]
	rdM1 := cfg["rd_architekt_r1_mies_zl"]
	rdM2 := cfg["rd_architekt_r2_mies_zl"]
	rdM3 := cfg["rd_architekt_r3_mies_zl"]

	bdCommPct := cfg["prowizja_bd_b2b_proc"]
	bdRetM1 := cfg["bd_retainer_r1_mies_zl"]
	bdRetM2 := cfg["bd_retainer_r2_mies_zl"]
	bdRetM3 := cfg["bd_retainer_r3_mies_zl"]

	founderNetMonthly := cfg["wyplata_zalozyciela_netto_mies_zl"]
	officeMonthly := cfg["koszty_biuro_ksiegowosc_mies_zl"]
	officeYearly := officeMonthly * 12.0
	citPct := cfg["podatek_cit_efektywny_proc"]

	origPrice := cfg["cena_originals_zl"]
	origMargin := cfg["marza_originals_szt_zl"]
	bookPrice := cfg["cena_ksiazki_zl"]
	feePct := cfg["prowizja_katalog_proc"]
	authorPct := 100.0 - feePct
	bookFee := bookPrice * (feePct / 100.0)
	authorShare := bookPrice - bookFee

	indiePrice := cfg["cena_indie_zl"]
	proPrice := cfg["cena_pro_zl"]
	uniPrice := cfg["cena_uczelnia_zl"]
	varCostPct := cfg["koszt_zmienny_proc"]

	srcGrants := cfg["zrodlo_dotacje_zl"]
	srcInvestors := cfg["zrodlo_inwestorzy_zl"]
	srcCrowdfunding := cfg["zrodlo_crowdfunding_zl"]
	srcOwnSales := cfg["zrodlo_sprzedaz_wlasna_zl"]
	totalBudget := srcGrants + srcInvestors + srcCrowdfunding + srcOwnSales
	nonDilutive := srcGrants + srcCrowdfunding + srcOwnSales
	nonDilutivePct := int(math.Round((nonDilutive / totalBudget) * 100.0))

	minTicket := cfg["bilet_inwestora_min_zl"]
	minTicketPct := invEquityPct * (minTicket / srcInvestors)

	budPremiery := cfg["koszt_aktorzy_zl"] + cfg["koszt_studio_dzwiek_zl"] + cfg["koszt_autorzy_zl"] + cfg["koszt_ilustracje_sfx_zl"]
	budIT := cfg["koszt_audyt_cert_zl"] + cfg["koszt_mobile_dev_zl"] + cfg["koszt_sklep_infra_zl"] + cfg["koszt_prawo_umowy_zl"]
	budRunway := cfg["koszt_pas_startowy_zl"]
	budRozwojTotal := budIT + budRunway
	budSprzedaz := cfg["koszt_kampania_cf_zl"] + cfg["koszt_promocja_premier_zl"] + cfg["koszt_warsztaty_b2b_zl"] + cfg["koszt_targi_konkursy_zl"]

	multMin := cfg["mnoznik_wyceny_min"]
	multMax := cfg["mnoznik_wyceny_max"]

	calcYr := func(yr int, rdMonthly, bdRetMonthly float64) deckYearCalc {
		var r deckYearCalc
		u := cfg[fmt.Sprintf("s_r%d_uczelnie", yr)]
		ind := cfg[fmt.Sprintf("s_r%d_indie", yr)]
		pro := cfg[fmt.Sprintf("s_r%d_pro", yr)]
		oT := cfg[fmt.Sprintf("s_r%d_originals_tytuly", yr)]
		oS := cfg[fmt.Sprintf("s_r%d_originals_szt", yr)]
		cT := cfg[fmt.Sprintf("s_r%d_katalog_tytuly", yr)]
		cS := cfg[fmt.Sprintf("s_r%d_katalog_szt", yr)]

		r.origUnits = oT * oS
		r.catUnits = cT * cS
		r.revUni = u * uniPrice
		r.revStudio = ind*indiePrice + pro*proPrice
		r.revLic = r.revUni + r.revStudio
		r.revOrig = r.origUnits * origMargin
		r.revCat = r.catUnits * bookFee
		r.revTotal = r.revLic + r.revOrig + r.revCat

		r.varCosts = math.Round((r.revLic + r.revCat) * (varCostPct / 100.0))
		r.grossMargin = r.revTotal - r.varCosts

		r.rdArchYearly = math.Round(rdMonthly * 12.0)
		r.bdRetYearly = math.Round(bdRetMonthly * 12.0)
		r.bdCommYearly = math.Round(r.revLic * (bdCommPct / 100.0))
		r.bdTotalYearly = r.bdRetYearly + r.bdCommYearly
		r.officeYearly = math.Round(officeYearly)
		r.bdAndAdminYearly = r.bdTotalYearly + r.officeYearly
		r.opexTotal = r.rdArchYearly + r.bdAndAdminYearly

		r.ebitdaCompany = math.Max(0, r.grossMargin-r.opexTotal)
		r.citTax = math.Round(r.ebitdaCompany * (citPct / 100.0))
		r.netProfit = r.ebitdaCompany - r.citTax

		r.archDivYearly = math.Round(r.netProfit * (archEquityPct / 100.0))
		r.bdDivYearly = math.Round(r.netProfit * (bdEquityPct / 100.0))
		r.invDivYearly = r.netProfit - r.archDivYearly - r.bdDivYearly

		r.archTotalYearly = r.rdArchYearly + r.archDivYearly
		r.archTotalMonthly = math.Round(r.archTotalYearly / 12.0)

		r.bdTotalCashYearly = r.bdTotalYearly + r.bdDivYearly
		r.bdTotalCashMonthly = math.Round(r.bdTotalCashYearly / 12.0)
		return r
	}

	y1 := calcYr(1, rdM1, bdRetM1)
	y2 := calcYr(2, rdM2, bdRetM2)
	y3 := calcYr(3, rdM3, bdRetM3)

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

	valMin := y3.revTotal * multMin
	valMax := y3.revTotal * multMax
	stakeMin := valMin * (invEquityPct / 100.0)
	stakeMax := valMax * (invEquityPct / 100.0)
	archStakeMin := valMin * (archEquityPct / 100.0)
	archStakeMax := valMax * (archEquityPct / 100.0)
	bdStakeMin := valMin * (bdEquityPct / 100.0)
	bdStakeMax := valMax * (bdEquityPct / 100.0)

	roiMin := (invDividend3Y + stakeMin) / srcInvestors
	roiMax := (invDividend3Y + stakeMax) / srcInvestors

	// Monthly break-even units in Year 1 (covering R&D Architect + BD Retainer + Office = 20k + 3k + 2k = 25k PLN/m)
	bepMonthlyOPEX := rdM1 + bdRetM1 + officeMonthly
	bepMonthlyUnits := int64(math.Ceil(bepMonthlyOPEX / origMargin))
	bepDailyUnits := int64(math.Ceil(float64(bepMonthlyUnits) / 30.0))

	pctBudPremiery := int(math.Round((budPremiery / totalBudget) * 100.0))
	pctBudRozwoj := int(math.Round((budRozwojTotal / totalBudget) * 100.0))
	pctBudSprzedaz := 100 - pctBudPremiery - pctBudRozwoj

	monthsY1, monthsY2, monthsY3, allMonths := build36MonthSimulation(
		y1, y2, y3,
		origMargin, varCostPct, bdCommPct, citPct,
		archEquityPct, bdEquityPct,
	)

	writeCalculatedFinanceCSV(rootDir, cfg,
		archEquityPct, bdEquityPct, invEquityPct,
		origPrice, origMargin, bookPrice, bookFee, indiePrice, proPrice, uniPrice,
		totalBudget, srcGrants, srcInvestors, srcCrowdfunding, srcOwnSales, nonDilutive,
		minTicket, minTicketPct, budPremiery, budIT, budRunway, budSprzedaz,
		y1, y2, y3,
		rev3Y, ebitda3Y, netProfit3Y, archTotal3Y, bdTotal3Y, invDividend3Y,
		valMin, valMax, stakeMin, stakeMax, archStakeMin, archStakeMax, bdStakeMin, bdStakeMax, roiMin, roiMax,
	)
	write36MonthSimulationCSV(rootDir, allMonths, y1, y2, y3, rev3Y, ebitda3Y, netProfit3Y, archTotal3Y, bdTotal3Y, invDividend3Y)

	vars := map[string]any{
		"udzial_architekta_proc": fmtPolishInt(int64(archEquityPct)),
		"udzial_biznes_proc":     fmtPolishInt(int64(bdEquityPct)),
		"udzial_zespolu_proc":    fmtPolishInt(int64(teamEquityPct)),
		"udzial_inwestora_proc":  fmtPolishInt(int64(invEquityPct)),
		"udzial_zalozyciela_proc": fmtPolishInt(int64(teamEquityPct)),

		"stawka_architekta_godz_zl": fmtPolishInt(int64(archHourly)),
		"rd_arch_r1_mies_zl":        fmtPolishInt(int64(rdM1)),
		"rd_arch_r2_mies_zl":        fmtPolishInt(int64(rdM2)),
		"rd_arch_r3_mies_zl":        fmtPolishInt(int64(rdM3)),
		"rd_arch_r1_rok_zl":         fmtPolishInt(int64(y1.rdArchYearly)),
		"rd_arch_r2_rok_zl":         fmtPolishInt(int64(y2.rdArchYearly)),
		"rd_arch_r3_rok_zl":         fmtPolishInt(int64(y3.rdArchYearly)),
		"rd_arch_3lata_zl":          fmtPolishInt(int64(rdArch3Y)),

		"prowizja_bd_b2b_proc":  fmtPolishInt(int64(bdCommPct)),
		"bd_ret_r1_mies_zl":     fmtPolishInt(int64(bdRetM1)),
		"bd_ret_r2_mies_zl":     fmtPolishInt(int64(bdRetM2)),
		"bd_ret_r3_mies_zl":     fmtPolishInt(int64(bdRetM3)),
		"bd_ops_r1_rok_zl":      fmtPolishInt(int64(y1.bdTotalYearly)),
		"bd_ops_r2_rok_zl":      fmtPolishInt(int64(y2.bdTotalYearly)),
		"bd_ops_r3_rok_zl":      fmtPolishInt(int64(y3.bdTotalYearly)),
		"bd_ops_3lata_zl":       fmtPolishInt(int64(bdOps3Y)),
		"bd_admin_r1_rok_zl":    fmtPolishInt(int64(y1.bdAndAdminYearly)),
		"bd_admin_r2_rok_zl":    fmtPolishInt(int64(y2.bdAndAdminYearly)),
		"bd_admin_r3_rok_zl":    fmtPolishInt(int64(y3.bdAndAdminYearly)),

		"arch_dyw_r1_zl":        fmtPolishInt(int64(y1.archDivYearly)),
		"arch_dyw_r2_zl":        fmtPolishInt(int64(y2.archDivYearly)),
		"arch_dyw_r3_zl":        fmtPolishInt(int64(y3.archDivYearly)),
		"arch_dyw_3lata_zl":     fmtPolishInt(int64(archDiv3Y)),
		"arch_razem_r1_rok_zl":  fmtPolishInt(int64(y1.archTotalYearly)),
		"arch_razem_r2_rok_zl":  fmtPolishInt(int64(y2.archTotalYearly)),
		"arch_razem_r3_rok_zl":  fmtPolishInt(int64(y3.archTotalYearly)),
		"arch_razem_r1_mies_zl": fmtPolishInt(int64(y1.archTotalMonthly)),
		"arch_razem_r2_mies_zl": fmtPolishInt(int64(y2.archTotalMonthly)),
		"arch_razem_r3_mies_zl": fmtPolishInt(int64(y3.archTotalMonthly)),
		"arch_suma_3lata_zl":    fmtPolishInt(int64(archTotal3Y)),
		"arch_wycena_r3_mln":    fmt.Sprintf("%s–%s mln zł", fmtPolishDec(archStakeMin/1000000.0, 2), fmtPolishDec(archStakeMax/1000000.0, 2)),

		"bd_dyw_r1_zl":          fmtPolishInt(int64(y1.bdDivYearly)),
		"bd_dyw_r2_zl":          fmtPolishInt(int64(y2.bdDivYearly)),
		"bd_dyw_r3_zl":          fmtPolishInt(int64(y3.bdDivYearly)),
		"bd_dyw_3lata_zl":       fmtPolishInt(int64(bdDiv3Y)),
		"bd_razem_r1_rok_zl":    fmtPolishInt(int64(y1.bdTotalCashYearly)),
		"bd_razem_r2_rok_zl":    fmtPolishInt(int64(y2.bdTotalCashYearly)),
		"bd_razem_r3_rok_zl":    fmtPolishInt(int64(y3.bdTotalCashYearly)),
		"bd_razem_r1_mies_zl":   fmtPolishInt(int64(y1.bdTotalCashMonthly)),
		"bd_razem_r2_mies_zl":   fmtPolishInt(int64(y2.bdTotalCashMonthly)),
		"bd_razem_r3_mies_zl":   fmtPolishInt(int64(y3.bdTotalCashMonthly)),
		"bd_suma_3lata_zl":      fmtPolishInt(int64(bdTotal3Y)),
		"bd_wycena_r3_mln":      fmt.Sprintf("%s–%s mln zł", fmtPolishDec(bdStakeMin/1000000.0, 2), fmtPolishDec(bdStakeMax/1000000.0, 2)),

		"inv_dyw_r1_zl":         fmtPolishInt(int64(y1.invDivYearly)),
		"inv_dyw_r2_zl":         fmtPolishInt(int64(y2.invDivYearly)),
		"inv_dyw_r3_zl":         fmtPolishInt(int64(y3.invDivYearly)),

		"bep_mies_opex_zl":      fmtPolishInt(int64(bepMonthlyOPEX)),
		"bep_mies_szt":          fmtPolishInt(bepMonthlyUnits),
		"bep_dzien_szt":         fmtPolishInt(bepDailyUnits),

		"zalozyciel_netto_mies_zl": fmtPolishInt(int64(founderNetMonthly)),
		"zalozyciel_netto_rok_zl":  fmtPolishInt(int64(founderNetMonthly * 12)),
		"zalozyciel_netto_m_r1":    fmtPolishInt(int64(y1.archTotalMonthly)),
		"zalozyciel_netto_m_r2":    fmtPolishInt(int64(y2.archTotalMonthly)),
		"zalozyciel_netto_m_r3":    fmtPolishInt(int64(y3.archTotalMonthly)),
		"zalozyciel_suma_3lata_zl": fmtPolishInt(int64(archTotal3Y)),

		"cena_originals_zl":      fmtPolishInt(int64(origPrice)),
		"marza_originals_szt_zl": fmtPolishInt(int64(origMargin)),
		"cena_ksiazki_zl":        fmtPolishInt(int64(bookPrice)),
		"prowizja_katalog_proc":  fmtPolishInt(int64(feePct)),
		"udzial_autora_proc":     fmtPolishInt(int64(authorPct)),
		"kwota_autora_zl":        fmtPolishInt(int64(authorShare)),
		"prowizja_szt_zl":        fmtPolishInt(int64(bookFee)),
		"cena_indie_zl":          fmtPolishInt(int64(indiePrice)),
		"cena_pro_zl":            fmtPolishInt(int64(proPrice)),
		"cena_uczelnia_zl":       fmtPolishInt(int64(uniPrice)),
		"koszty_biuro_mies_zl":   fmtPolishInt(int64(officeMonthly)),

		"budzet_caly_zl":          fmtPolishInt(int64(totalBudget)),
		"budzet_caly_tys":         fmtPolishInt(int64(totalBudget / 1000.0)),
		"zrodlo_dotacje_zl":       fmtPolishInt(int64(srcGrants)),
		"zrodlo_dotacje_tys":      fmtPolishInt(int64(srcGrants / 1000.0)),
		"zrodlo_inwestorzy_zl":    fmtPolishInt(int64(srcInvestors)),
		"zrodlo_inwestorzy_tys":   fmtPolishInt(int64(srcInvestors / 1000.0)),
		"zrodlo_crowdfunding_zl":  fmtPolishInt(int64(srcCrowdfunding)),
		"zrodlo_crowdfunding_tys": fmtPolishInt(int64(srcCrowdfunding / 1000.0)),
		"zrodlo_wlasne_zl":        fmtPolishInt(int64(srcOwnSales)),
		"zrodlo_wlasne_tys":       fmtPolishInt(int64(srcOwnSales / 1000.0)),
		"bezzwrotne_zl":           fmtPolishInt(int64(nonDilutive)),
		"bezzwrotne_tys":          fmtPolishInt(int64(nonDilutive / 1000.0)),
		"bezzwrotne_proc":         strconv.Itoa(nonDilutivePct),

		"kwota_inwestycji_zl":    fmtPolishInt(int64(srcInvestors)),
		"kwota_inwestycji_tys":   fmtPolishInt(int64(srcInvestors / 1000.0)),
		"bilet_inwestora_min_zl": fmtPolishInt(int64(minTicket)),
		"bilet_inwestora_proc":   fmtPolishDec(minTicketPct, 2),

		"budzet_premiery_zl":     fmtPolishInt(int64(budPremiery)),
		"budzet_premiery_tys":    fmtPolishInt(int64(budPremiery / 1000.0)),
		"budzet_it_zl":           fmtPolishInt(int64(budIT)),
		"budzet_it_tys":          fmtPolishInt(int64(budIT / 1000.0)),
		"budzet_pas_startowy_zl": fmtPolishInt(int64(budRunway)),
		"budzet_rozwoj_zl":       fmtPolishInt(int64(budRozwojTotal)),
		"budzet_rozwoj_tys":      fmtPolishInt(int64(budRozwojTotal / 1000.0)),
		"budzet_sprzedaz_zl":     fmtPolishInt(int64(budSprzedaz)),
		"budzet_sprzedaz_tys":    fmtPolishInt(int64(budSprzedaz / 1000.0)),
		"budzet_premiery_proc":   strconv.Itoa(pctBudPremiery),
		"budzet_rozwoj_proc":     strconv.Itoa(pctBudRozwoj),
		"budzet_sprzedaz_proc":   strconv.Itoa(pctBudSprzedaz),

		"koszt_aktorzy_zl":          fmtPolishInt(int64(cfg["koszt_aktorzy_zl"])),
		"koszt_studio_dzwiek_zl":    fmtPolishInt(int64(cfg["koszt_studio_dzwiek_zl"])),
		"koszt_autorzy_zl":          fmtPolishInt(int64(cfg["koszt_autorzy_zl"])),
		"koszt_ilustracje_sfx_zl":   fmtPolishInt(int64(cfg["koszt_ilustracje_sfx_zl"])),
		"koszt_audyt_cert_zl":       fmtPolishInt(int64(cfg["koszt_audyt_cert_zl"])),
		"koszt_mobile_dev_zl":       fmtPolishInt(int64(cfg["koszt_mobile_dev_zl"])),
		"koszt_sklep_infra_zl":      fmtPolishInt(int64(cfg["koszt_sklep_infra_zl"])),
		"koszt_prawo_umowy_zl":      fmtPolishInt(int64(cfg["koszt_prawo_umowy_zl"])),
		"koszt_kampania_cf_zl":      fmtPolishInt(int64(cfg["koszt_kampania_cf_zl"])),
		"koszt_promocja_premier_zl": fmtPolishInt(int64(cfg["koszt_promocja_premier_zl"])),
		"koszt_warsztaty_b2b_zl":    fmtPolishInt(int64(cfg["koszt_warsztaty_b2b_zl"])),
		"koszt_targi_konkursy_zl":   fmtPolishInt(int64(cfg["koszt_targi_konkursy_zl"])),
		"koszt_aktorzy_tys":         fmtPolishInt(int64(cfg["koszt_aktorzy_zl"] / 1000.0)),
		"koszt_studio_dzwiek_tys":   fmtPolishInt(int64(cfg["koszt_studio_dzwiek_zl"] / 1000.0)),
		"koszt_autorzy_tys":         fmtPolishInt(int64(cfg["koszt_autorzy_zl"] / 1000.0)),
		"koszt_ilustracje_sfx_tys":  fmtPolishInt(int64(cfg["koszt_ilustracje_sfx_zl"] / 1000.0)),
		"koszt_mobile_dev_tys":      fmtPolishInt(int64(cfg["koszt_mobile_dev_zl"] / 1000.0)),
		"koszt_studio_dev_tys":      fmtPolishInt(int64(cfg["koszt_audyt_cert_zl"] / 1000.0)),

		"s_r1_uczelnie":       fmtPolishInt(int64(cfg["s_r1_uczelnie"])),
		"s_r1_indie":          fmtPolishInt(int64(cfg["s_r1_indie"])),
		"s_r1_pro":            fmtPolishInt(int64(cfg["s_r1_pro"])),
		"s_r1_orig_tyt":       fmtPolishInt(int64(cfg["s_r1_originals_tytuly"])),
		"s_r1_orig_szt":       fmtPolishInt(int64(cfg["s_r1_originals_szt"])),
		"s_r1_orig_suma_szt":  fmtPolishInt(int64(y1.origUnits)),
		"s_r1_kat_tyt":        fmtPolishInt(int64(cfg["s_r1_katalog_tytuly"])),
		"s_r1_kat_suma_szt":   fmtPolishInt(int64(y1.catUnits)),
		"przychod_orig_r1_zl": fmtPolishInt(int64(y1.revOrig)),
		"przychod_kat_r1_zl":  fmtPolishInt(int64(y1.revCat)),
		"przychod_lic_r1_zl":  fmtPolishInt(int64(y1.revLic)),
		"przychod_s_r1_zl":    fmtPolishInt(int64(y1.revTotal)),
		"ebitda_r1_zl":        fmtPolishInt(int64(y1.ebitdaCompany)),
		"nadwyzka_r1_zl":      fmtPolishInt(int64(y1.netProfit)),

		"s_r2_uczelnie":       fmtPolishInt(int64(cfg["s_r2_uczelnie"])),
		"s_r2_indie":          fmtPolishInt(int64(cfg["s_r2_indie"])),
		"s_r2_pro":            fmtPolishInt(int64(cfg["s_r2_pro"])),
		"s_r2_orig_tyt":       fmtPolishInt(int64(cfg["s_r2_originals_tytuly"])),
		"s_r2_orig_szt":       fmtPolishInt(int64(cfg["s_r2_originals_szt"])),
		"s_r2_orig_suma_szt":  fmtPolishInt(int64(y2.origUnits)),
		"s_r2_kat_tyt":        fmtPolishInt(int64(cfg["s_r2_katalog_tytuly"])),
		"s_r2_kat_suma_szt":   fmtPolishInt(int64(y2.catUnits)),
		"przychod_orig_r2_zl": fmtPolishInt(int64(y2.revOrig)),
		"przychod_kat_r2_zl":  fmtPolishInt(int64(y2.revCat)),
		"przychod_lic_r2_zl":  fmtPolishInt(int64(y2.revLic)),
		"przychod_s_r2_zl":    fmtPolishInt(int64(y2.revTotal)),
		"ebitda_r2_zl":        fmtPolishInt(int64(y2.ebitdaCompany)),
		"nadwyzka_r2_zl":      fmtPolishInt(int64(y2.netProfit)),

		"s_r3_uczelnie":       fmtPolishInt(int64(cfg["s_r3_uczelnie"])),
		"s_r3_indie":          fmtPolishInt(int64(cfg["s_r3_indie"])),
		"s_r3_pro":            fmtPolishInt(int64(cfg["s_r3_pro"])),
		"s_r3_orig_tyt":       fmtPolishInt(int64(cfg["s_r3_originals_tytuly"])),
		"s_r3_orig_szt":       fmtPolishInt(int64(cfg["s_r3_originals_szt"])),
		"s_r3_orig_suma_szt":  fmtPolishInt(int64(y3.origUnits)),
		"s_r3_kat_tyt":        fmtPolishInt(int64(cfg["s_r3_katalog_tytuly"])),
		"s_r3_kat_suma_szt":   fmtPolishInt(int64(y3.catUnits)),
		"przychod_orig_r3_zl": fmtPolishInt(int64(y3.revOrig)),
		"przychod_kat_r3_zl":  fmtPolishInt(int64(y3.revCat)),
		"przychod_lic_r3_zl":  fmtPolishInt(int64(y3.revLic)),
		"przychod_s_r3_zl":    fmtPolishInt(int64(y3.revTotal)),
		"ebitda_r3_zl":        fmtPolishInt(int64(y3.ebitdaCompany)),
		"nadwyzka_r3_zl":      fmtPolishInt(int64(y3.netProfit)),

		"przychod_3lata_zl": fmtPolishInt(int64(rev3Y)),
		"zysk_s_r1_tys":     fmtPolishInt(int64(math.Round(y1.ebitdaCompany / 1000.0))),
		"zysk_s_r3_mln":     fmtPolishDec(y3.ebitdaCompany/1000000.0, 2),
		"ebitda_3lata":      fmtPolishInt(int64(ebitda3Y)),
		"nadwyzka_3lata":    fmtPolishInt(int64(netProfit3Y)),

		"zysk_inwestora_3lata_zl": fmtPolishInt(int64(invDividend3Y)),
		"wycena_spolki_r3_mln":    fmt.Sprintf("%s–%s mln zł", fmtPolishDec(valMin/1000000.0, 2), fmtPolishDec(valMax/1000000.0, 2)),
		"wartosc_udzialow_r3_tys": fmt.Sprintf("%s–%s mln zł", fmtPolishDec(stakeMin/1000000.0, 2), fmtPolishDec(stakeMax/1000000.0, 2)),
		"zwrot_inwestora_x":       fmt.Sprintf("%s× – %s×", fmtPolishDec(roiMin, 1), fmtPolishDec(roiMax, 1)),

		"months_y1": monthsY1,
		"months_y2": monthsY2,
		"months_y3": monthsY3,
	}

	return NewContext(vars)
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
	y1, y2, y3 deckYearCalc,
	origMargin, varCostPct, bdCommPct, citPct float64,
	archEquityPct, bdEquityPct float64,
) ([]map[string]any, []map[string]any, []map[string]any, []deckMonthRow) {
	milestones := [36]string{
		"Audyt kodu + nagranie Pilota #1 (1h)",
		"Premiera darmowego Pilota #1 + Wspieram.to",
		"Pierwsze 2 uczelnie B2B + zamknięcie 2 biletów",
		"Nagranie Pilota #2 + wniosek CRPK/MKiDN",
		"Premiera #1 (Steam / iOS / Android) + 2 uczelnie",
		"Pełny rozruch sprzedaży #1 + katalog Indie",
		"Premiera #2 (Steam / Mobile) + wakacyjny pik",
		"Dystrybucja #1–#2 + przygotowanie jesieni",
		"Premiera #3 + 2 nowe uczelnie (rok akademicki)",
		"Steam Next Fest + warsztaty scenariuszowe",
		"Premiera #4 + Black Friday / Cyber Monday",
		"Szczyt świąteczny (4 tytuły Originals w sprzedaży)",

		"Nowy rok akademicki + start prac nad EN",
		"Premiera #5 (Steam PL/EN + App Store)",
		"Wdrożenia B2B na uczelniach (semestr letni)",
		"Wiosenna wyprzedaż Steam + rozwój katalogu",
		"Premiera #6 (Wspieram.to + Steam + Mobile)",
		"Letni Festiwal Steam + 6 tytułów w rotacji",
		"Wakacyjny pik audiobooków (tryb Auto-Choice)",
		"Premiera #7 + rozbudowa katalogu zewnętrznego",
		"Kampania B2B (12 uczelni aktywnych)",
		"Premiera #8 + jesienny Steam Next Fest",
		"Wyprzedaż jesienna Steam + Black Friday",
		"Szczyt Q4 (8 tytułów Originals + 15 z katalogu)",

		"Ekspansja UE (szkoły filmowe CEE/UE)",
		"Premiera #9 (równoczesny debiut PL + EN)",
		"Wiosenne odnowienia licencji B2B i Studio Pro",
		"Premiera #10 + akcja promocyjna u podcasterów",
		"25 tytułów zewnętrznych w czytniku Mojra",
		"Letni pik mobilny (iOS / Android / CarPlay)",
		"Premiera #11 (superprodukcja wieloobsadowa)",
		"Wakacyjna sprzedaż globalna na Steam EN",
		"Start roku akademickiego (22 uczelnie B2B)",
		"Premiera #12 + 35 tytułów w katalogu",
		"Globalny Black Friday (12 tytułów Originals)",
		"Zamknięcie 3. roku i gotowość dywidendowa/Exit",
	}

	// Weights for Year 1..3 reflecting seasonal & launch ramp-up
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
					netP = int64(math.Round(float64(ebitda) * (1.0 - citPct/100.0)))
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
				archDiv = int64(math.Round(float64(netP) * (archEquityPct / 100.0)))
				bdDiv = int64(math.Round(float64(netP) * (bdEquityPct / 100.0)))
				invDiv = netP - archDiv - bdDiv
			}
			sumArchDiv += archDiv
			sumBDDiv += bdDiv

			archTot := rdArr[i] + archDiv
			bdTot := bdCost + bdDiv

			code := fmt.Sprintf("M%02d", mNum)
			row := deckMonthRow{
				MonthNum:  mNum,
				Code:      code,
				Milestone: milestones[mNum-1],
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
				"code":       code,
				"milestone":  row.Milestone,
				"orig_szt":   fmtPolishInt(row.OrigUnits),
				"rev_orig":   fmtPolishInt(row.RevOrig),
				"rev_lic":    fmtPolishInt(row.RevLic),
				"rev_cat":    fmtPolishInt(row.RevCat),
				"rev_total":  fmtPolishInt(row.RevTotal),
				"rd_arch":    fmtPolishInt(row.RDArch),
				"bd_cost":    fmtPolishInt(row.BDCost),
				"ebitda":     fmtPolishInt(row.EBITDA),
				"net_profit": fmtPolishInt(row.NetProfit),
				"arch_total": fmtPolishInt(row.ArchTotal),
				"bd_total":   fmtPolishInt(row.BDTotal),
				"inv_div":    fmtPolishInt(row.InvDiv),
			})
		}
		return maps, rows
	}

	mY1, rY1 := buildYearMonths(0, y1, wOrigY1, wLicY1, wCatY1, wRDY1, wBDRetY1)
	mY2, rY2 := buildYearMonths(1, y2, wOrigY2, wLicY2, wCatY2, wUniform, wUniform)
	mY3, rY3 := buildYearMonths(2, y3, wOrigY3, wLicY3, wCatY3, wUniform, wUniform)

	var all []deckMonthRow
	all = append(all, rY1...)
	all = append(all, rY2...)
	all = append(all, rY3...)
	return mY1, mY2, mY3, all
}

func csvCell(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func writeCSVRow(sb *strings.Builder, c1, c2, c3, c4, c5 string) {
	sb.WriteString(csvCell(c1))
	sb.WriteByte(',')
	sb.WriteString(csvCell(c2))
	sb.WriteByte(',')
	sb.WriteString(csvCell(c3))
	sb.WriteByte(',')
	sb.WriteString(csvCell(c4))
	sb.WriteByte(',')
	sb.WriteString(csvCell(c5))
	sb.WriteByte('\n')
}

func writeCalculatedFinanceCSV(
	rootDir string,
	cfg map[string]float64,
	archEquityPct, bdEquityPct, invEquityPct float64,
	origPrice, origMargin, bookPrice, bookFee, indiePrice, proPrice, uniPrice float64,
	totalBudget, srcGrants, srcInvestors, srcCrowdfunding, srcOwnSales, nonDilutive float64,
	minTicket, minTicketPct, budPremiery, budIT, budRunway, budSprzedaz float64,
	y1, y2, y3 deckYearCalc,
	rev3Y, ebitda3Y, netProfit3Y, archTotal3Y, bdTotal3Y, invDividend3Y float64,
	valMin, valMax, stakeMin, stakeMax, archStakeMin, archStakeMax, bdStakeMin, bdStakeMax, roiMin, roiMax float64,
) {
	docsDir := filepath.Join(rootDir, "docs")
	if info, err := os.Stat(docsDir); err != nil || !info.IsDir() {
		return
	}
	var sb strings.Builder
	writeCSVRow(&sb, "Sekcja / Pozycja", "Rok 1", "Rok 2", "Rok 3", "Suma 3 Lat / Szczegóły (Źródło: docs/finanse.cfg)")

	writeCSVRow(&sb, "0. PODZIAŁ TORTU — KTO ILE DOSTAJE (GOTÓWKA + WYCENA UDZIAŁÓW)", "Rok 1", "Rok 2", "Rok 3", "Pełna zgodność z Deckiem Inwestorskim i symulacją M1–M36")
	writeCSVRow(&sb, fmt.Sprintf("A. Ty — Główny Architekt (%d%% udziałów): Faktura B2B R&D + Dywidenda", int(archEquityPct)), fmtPolishInt(int64(y1.archTotalYearly))+" zł ("+fmtPolishInt(int64(y1.archTotalMonthly))+" zł/m)", fmtPolishInt(int64(y2.archTotalYearly))+" zł ("+fmtPolishInt(int64(y2.archTotalMonthly))+" zł/m)", fmtPolishInt(int64(y3.archTotalYearly))+" zł ("+fmtPolishInt(int64(y3.archTotalMonthly))+" zł/m)", fmt.Sprintf("Gotówka 3 lata: %s zł + wycena %d%%: %s–%s mln zł", fmtPolishInt(int64(archTotal3Y)), int(archEquityPct), fmtPolishDec(archStakeMin/1000000.0, 2), fmtPolishDec(archStakeMax/1000000.0, 2)))
	writeCSVRow(&sb, fmt.Sprintf("B. Przyjaciel — Co-Founder Biznes/BD (%d%% udziałów): Retainer + 15%% Prowizji B2B + Dywidenda", int(bdEquityPct)), fmtPolishInt(int64(y1.bdTotalCashYearly))+" zł ("+fmtPolishInt(int64(y1.bdTotalCashMonthly))+" zł/m)", fmtPolishInt(int64(y2.bdTotalCashYearly))+" zł ("+fmtPolishInt(int64(y2.bdTotalCashMonthly))+" zł/m)", fmtPolishInt(int64(y3.bdTotalCashYearly))+" zł ("+fmtPolishInt(int64(y3.bdTotalCashMonthly))+" zł/m)", fmt.Sprintf("Gotówka 3 lata: %s zł + wycena %d%% (vesting 36M): %s–%s mln zł", fmtPolishInt(int64(bdTotal3Y)), int(bdEquityPct), fmtPolishDec(bdStakeMin/1000000.0, 2), fmtPolishDec(bdStakeMax/1000000.0, 2)))
	writeCSVRow(&sb, fmt.Sprintf("C. Inwestorzy Seed (%d%% udziałów za %s zł)", int(invEquityPct), fmtPolishInt(int64(srcInvestors))), fmtPolishInt(int64(y1.invDivYearly))+" zł dywidendy", fmtPolishInt(int64(y2.invDivYearly))+" zł dywidendy", fmtPolishInt(int64(y3.invDivYearly))+" zł dywidendy", fmt.Sprintf("Dywidenda 3 lata: %s zł + wycena %d%%: %s–%s mln zł (zwrot %s×–%s×)", fmtPolishInt(int64(invDividend3Y)), int(invEquityPct), fmtPolishDec(stakeMin/1000000.0, 2), fmtPolishDec(stakeMax/1000000.0, 2), fmtPolishDec(roiMin, 1), fmtPolishDec(roiMax, 1)))

	writeCSVRow(&sb, "1. ŹRÓDŁA BUDŻETU STARTOWEGO", fmtPolishInt(int64(totalBudget))+" zł", "100% budżetu", fmt.Sprintf("%s zł bez rozwodnienia udziałów", fmtPolishInt(int64(nonDilutive))), "4 niezależne źródła finansowania")
	writeCSVRow(&sb, "A. Dotacje rządowe (CRPK / MKiDN / PARP Dostępność)", fmtPolishInt(int64(srcGrants))+" zł", "33% budżetu", "0% oddanych udziałów", "Granty na narzędzia kreatywne, R&D i dostępność audio (TRL 7)")
	writeCSVRow(&sb, "B. Inwestorzy Seed (2 bilety po 40 tys. zł)", fmtPolishInt(int64(srcInvestors))+" zł", "30% budżetu", fmt.Sprintf("%d%% udziałów (Zespół zachowuje %d%%!)", int(invEquityPct), int(archEquityPct+bdEquityPct)), "2 aniołów biznesu po 40 tys. zł (5% każdy)")
	writeCSVRow(&sb, "C. Przedsprzedaż po darmowych Pilotach (Wspieram.to)", fmtPolishInt(int64(srcCrowdfunding))+" zł", "26% budżetu", "0% oddanych udziałów", "Przedsprzedaż 4 premier po wypuszczeniu darmowych 1h Pilotów")
	writeCSVRow(&sb, "D. Pierwsze wdrożenia uczelniane B2B", fmtPolishInt(int64(srcOwnSales))+" zł", "11% budżetu", "0% oddanych udziałów", "Pierwsze 4 szkoły filmowe / uczelnie × 7 500 zł")

	writeCSVRow(&sb, "2. KOSZTORYS STARTOWY CAPEX", fmtPolishInt(int64(totalBudget))+" zł", "100% wydatków", "12 miesięcy", "Produkcja Pilot-First (150 tys.) + R&D/Audyt/Mobile (60 tys.) + Dystrybucja/B2B (60 tys.)")
	writeCSVRow(&sb, "A. Produkcja 4 własnych premier Mojra Originals (Pilot-First)", fmtPolishInt(int64(budPremiery))+" zł", "56%", "4 tytuły etapowane Pilotami", "Aktorzy (60 tys.) + Studio (40 tys.) + Autorzy (34 tys.) + Ilustracje/SFX (16 tys.)")
	writeCSVRow(&sb, "B. R&D Aplikacji: Architektura Mobile/Steam, Audyt i Płatności", fmtPolishInt(int64(budIT))+" zł", "22%", "Gotowy silnik Go/WebGL", "Pentest i certyfikaty (18 tys.) + R&D iOS/Android (24 tys.) + Płatności i umowy (18 tys.)")
	writeCSVRow(&sb, "C. Kampania Wspieram.to, promocja Steam/Mobile i akwizycja B2B", fmtPolishInt(int64(budSprzedaz))+" zł", "22%", "4 kampanie + uczelnie B2B", "Wideo CF (18 tys.) + Promocja premier (24 tys.) + Spotkania B2B i targi (18 tys.)")

	writeCSVRow(&sb, "3. RACHUNEK ZYSKÓW I STRAT SPÓŁKI (P&L ROK 1–3)", "Rok 1", "Rok 2", "Rok 3", "Suma 3 Lat / Opis pozycji")
	writeCSVRow(&sb, "Przychód 1: Własne produkcje Mojra Originals (32 zł marży/szt.)", fmtPolishInt(int64(y1.revOrig))+" zł", fmtPolishInt(int64(y2.revOrig))+" zł", fmtPolishInt(int64(y3.revOrig))+" zł", "Wydania własne na Steam, App Store, Google Play i Wspieram.to")
	writeCSVRow(&sb, "Przychód 2: Katalog zewnętrznych autorów (6 zł prowizji/szt.)", fmtPolishInt(int64(y1.revCat))+" zł", fmtPolishInt(int64(y2.revCat))+" zł", fmtPolishInt(int64(y3.revCat))+" zł", "15% prowizji platformowej od zewnętrznych twórców")
	writeCSVRow(&sb, "Przychód 3: Licencje (Uczelnie B2B 7 500 zł + Indie 249 zł + Pro 599 zł)", fmtPolishInt(int64(y1.revLic))+" zł", fmtPolishInt(int64(y2.revLic))+" zł", fmtPolishInt(int64(y3.revLic))+" zł", "Licencje kampusowe dla szkół filmowych oraz subskrypcje Playbook Studio")
	writeCSVRow(&sb, "PRZYCHÓD SPÓŁKI OGÓŁEM", fmtPolishInt(int64(y1.revTotal))+" zł", fmtPolishInt(int64(y2.revTotal))+" zł", fmtPolishInt(int64(y3.revTotal))+" zł", fmtPolishInt(int64(rev3Y))+" zł łącznie w 3 lata")
	writeCSVRow(&sb, "Koszt OPEX 1: R&D, architektura silnika i rozwój wieloplatformowy (B2B Architekt)", fmtPolishInt(int64(y1.rdArchYearly))+" zł", fmtPolishInt(int64(y2.rdArchYearly))+" zł", fmtPolishInt(int64(y3.rdArchYearly))+" zł", "Kontrakt B2B Głównego Architekta (IP Box 5%) ujęty w kosztach rozwoju oprogramowania")
	writeCSVRow(&sb, "Koszt OPEX 2: Rozwój biznesu B2B (Retainer BD + 15% prowizji) oraz księgowość", fmtPolishInt(int64(y1.bdAndAdminYearly))+" zł", fmtPolishInt(int64(y2.bdAndAdminYearly))+" zł", fmtPolishInt(int64(y3.bdAndAdminYearly))+" zł", "Wynagrodzenie i prowizje Co-Foundera Biznesowego (BD) + biuro rachunkowe (2 000 zł/m)")
	writeCSVRow(&sb, "ZYSK OPERACYJNY SPÓŁKI (EBITDA)", fmtPolishInt(int64(y1.ebitdaCompany))+" zł", fmtPolishInt(int64(y2.ebitdaCompany))+" zł", fmtPolishInt(int64(y3.ebitdaCompany))+" zł", fmtPolishInt(int64(ebitda3Y))+" zł łącznie w 3 lata")
	writeCSVRow(&sb, "ZYSK NETTO PO PODATKU CIT (DO PODZIAŁU DYWIDENDOWEGO)", fmtPolishInt(int64(y1.netProfit))+" zł", fmtPolishInt(int64(y2.netProfit))+" zł", fmtPolishInt(int64(y3.netProfit))+" zł", fmt.Sprintf("%s zł (podział: %d%% Architekt, %d%% Biznes/BD, %d%% Inwestorzy)", fmtPolishInt(int64(netProfit3Y)), int(archEquityPct), int(bdEquityPct), int(invEquityPct)))

	if err := os.WriteFile(filepath.Join(docsDir, "kalkulator-finansowy.csv"), []byte(sb.String()), 0644); err != nil {
		return
	}
}

func write36MonthSimulationCSV(
	rootDir string,
	allMonths []deckMonthRow,
	y1, y2, y3 deckYearCalc,
	rev3Y, ebitda3Y, netProfit3Y, archTotal3Y, bdTotal3Y, invDividend3Y float64,
) {
	docsDir := filepath.Join(rootDir, "docs")
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

	writeMultiCSVRow(
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
	)

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

	writeYearSum := func(label string, yc deckYearCalc) {
		writeMultiCSVRow(
			label,
			"Podsumowanie roczne",
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

	writeYearSum("SUMA ROK 1 (M01-M12)", y1)
	writeYearSum("SUMA ROK 2 (M13-M24)", y2)
	writeYearSum("SUMA ROK 3 (M25-M36)", y3)
	writeMultiCSVRow(
		"SUMA 3 LATA (M01-M36)",
		"Łącznie przez 36 miesięcy",
		fmtPolishInt(int64(y1.origUnits+y2.origUnits+y3.origUnits)),
		fmtPolishInt(int64(y1.revOrig+y2.revOrig+y3.revOrig)),
		fmtPolishInt(int64(y1.revLic+y2.revLic+y3.revLic)),
		fmtPolishInt(int64(y1.revCat+y2.revCat+y3.revCat)),
		fmtPolishInt(int64(rev3Y)),
		fmtPolishInt(int64(y1.rdArchYearly+y2.rdArchYearly+y3.rdArchYearly)),
		fmtPolishInt(int64(y1.bdTotalYearly+y2.bdTotalYearly+y3.bdTotalYearly)),
		fmtPolishInt(int64(y1.officeYearly+y2.officeYearly+y3.officeYearly)),
		fmtPolishInt(int64(ebitda3Y)),
		fmtPolishInt(int64(netProfit3Y)),
		fmtPolishInt(int64(archTotal3Y)),
		fmtPolishInt(int64(bdTotal3Y)),
		fmtPolishInt(int64(invDividend3Y)),
	)

	if err := os.WriteFile(filepath.Join(docsDir, "symulacja-36-miesiecy.csv"), []byte(sb.String()), 0644); err != nil {
		return
	}
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

// FindHeadlessPDFBrowser locates an installed Chromium-based browser for PDF rendering.
func FindHeadlessPDFBrowser() string {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "windows":
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")} {
			if base == "" {
				continue
			}
			candidates = append(candidates,
				filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(base, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			)
		}
	default:
		for _, bin := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "brave-browser", "microsoft-edge"} {
			if p, err := exec.LookPath(bin); err == nil && p != "" {
				return p
			}
		}
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func renderHTMLFileToPDF(browserBin, htmlPath, outPDFPath string) error {
	absHTML, err := filepath.Abs(htmlPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absHTML); err != nil {
		return fmt.Errorf("brak skompilowanego pliku %s: %w", absHTML, err)
	}
	absOutPDF, err := filepath.Abs(outPDFPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absOutPDF), 0755); err != nil {
		return err
	}
	tmpProfileDir, err := os.MkdirTemp("", "mojra-deck-pdf-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpProfileDir)

	fileURL := "file://" + filepath.ToSlash(absHTML)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		browserBin,
		"--headless=new",
		"--disable-gpu",
		"--disable-extensions",
		"--disable-background-networking",
		"--no-first-run",
		"--no-default-browser-check",
		"--user-data-dir="+tmpProfileDir,
		"--no-pdf-header-footer",
		"--print-to-pdf-no-header",
		"--print-to-pdf="+absOutPDF,
		fileURL,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("błąd generowania PDF (%v): %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// BuildDeckPDF renders dist/www/deck.html, dist/www/plan.html, and dist/www/przeglad.html
// into docs/mojra-deck-finansowy.pdf, docs/mojra-plan.pdf, and docs/mojra-przeglad-zalozyciela.pdf.
func BuildDeckPDF(rootDir, customOutPath string) (string, error) {
	browserBin := FindHeadlessPDFBrowser()
	if browserBin == "" {
		return "", fmt.Errorf("nie znaleziono przeglądarki Chromium/Chrome/Edge/Brave do wygenerowania PDF")
	}
	docsDir := filepath.Join(rootDir, "docs")
	_ = os.MkdirAll(docsDir, 0755)

	outPDF := filepath.Join(docsDir, "mojra-deck-finansowy.pdf")
	if strings.TrimSpace(customOutPath) != "" {
		outPDF = customOutPath
	}
	absOutPDF, err := filepath.Abs(outPDF)
	if err != nil {
		return "", err
	}
	deckHTMLPath := filepath.Join(rootDir, "dist", "www", "deck.html")
	if err := renderHTMLFileToPDF(browserBin, deckHTMLPath, absOutPDF); err != nil {
		return "", err
	}
	if raw, rErr := os.ReadFile(absOutPDF); rErr == nil {
		if err := os.WriteFile(filepath.Join(rootDir, "dist", "www", "mojra-deck-finansowy.pdf"), raw, 0644); err != nil {
			return "", err
		}
	}
	planHTMLPath := filepath.Join(rootDir, "dist", "www", "plan.html")
	if _, err := os.Stat(planHTMLPath); err == nil {
		docsPlanPDF := filepath.Join(docsDir, "mojra-plan.pdf")
		if err := renderHTMLFileToPDF(browserBin, planHTMLPath, docsPlanPDF); err == nil {
			if raw, rErr := os.ReadFile(docsPlanPDF); rErr == nil {
				if err := os.WriteFile(filepath.Join(rootDir, "dist", "www", "mojra-plan.pdf"), raw, 0644); err != nil {
					return "", err
				}
			}
		}
	}
	przegladHTMLPath := filepath.Join(rootDir, "dist", "www", "przeglad.html")
	if _, err := os.Stat(przegladHTMLPath); err == nil {
		docsPrzegladPDF := filepath.Join(docsDir, "mojra-przeglad-zalozyciela.pdf")
		if err := renderHTMLFileToPDF(browserBin, przegladHTMLPath, docsPrzegladPDF); err == nil {
			if raw, rErr := os.ReadFile(docsPrzegladPDF); rErr == nil {
				if err := os.WriteFile(filepath.Join(rootDir, "dist", "www", "mojra-przeglad-zalozyciela.pdf"), raw, 0644); err != nil {
					return "", err
				}
			}
		}
	}
	return absOutPDF, nil
}
