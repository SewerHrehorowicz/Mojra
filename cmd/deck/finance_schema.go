package main

const (
	financeCfgFile     = "finanse.cfg"
	financeCalcCSVFile = "kalkulator-finansowy.csv"
	financeSimCSVFile  = "symulacja-36-miesiecy.csv"

	fmtMonthCode   = "M%02d"
	fmtValRangeMln = "%s–%s mln zł"
	fmtROIRangeX   = "%s× – %s×"

	monthKeyCode      = "code"
	monthKeyMilestone = "milestone"
	monthKeyOrigUnits = "orig_szt"
	monthKeyRevOrig   = "rev_orig"
	monthKeyRevLic    = "rev_lic"
	monthKeyRevCat    = "rev_cat"
	monthKeyRevTotal  = "rev_total"
	monthKeyRDArch    = "rd_arch"
	monthKeyBDCost    = "bd_cost"
	monthKeyEBITDA    = "ebitda"
	monthKeyNetProfit = "net_profit"
	monthKeyArchTotal = "arch_total"
	monthKeyBDTotal   = "bd_total"
	monthKeyInvDiv    = "inv_div"

	ctxKeyArchEquityPct     = "udzial_architekta_proc"
	ctxKeyBDEquityPct       = "udzial_biznes_proc"
	ctxKeyTeamEquityPct     = "udzial_zespolu_proc"
	ctxKeyInvEquityPct      = "udzial_inwestora_proc"
	ctxKeyFounderEquityPct  = "udzial_zalozyciela_proc"
	ctxKeyArchHourlyRate    = "stawka_architekta_godz_zl"
	ctxKeyRDArch3Y          = "rd_arch_3lata_zl"
	ctxKeyBDCommPct         = "prowizja_bd_b2b_proc"
	ctxKeyBDOps3Y           = "bd_ops_3lata_zl"
	ctxKeyArchDiv3Y         = "arch_dyw_3lata_zl"
	ctxKeyArchSum3Y         = "arch_suma_3lata_zl"
	ctxKeyArchValR3Mln      = "arch_wycena_r3_mln"
	ctxKeyBDDiv3Y           = "bd_dyw_3lata_zl"
	ctxKeyBDSum3Y           = "bd_suma_3lata_zl"
	ctxKeyBDValR3Mln        = "bd_wycena_r3_mln"
	ctxKeyBEPMonthOPEX      = "bep_mies_opex_zl"
	ctxKeyBEPMonthUnits     = "bep_mies_szt"
	ctxKeyBEPDayUnits       = "bep_dzien_szt"
	ctxKeyFounderNetMonth   = "zalozyciel_netto_mies_zl"
	ctxKeyFounderNetYear    = "zalozyciel_netto_rok_zl"
	ctxKeyFounderSum3Y      = "zalozyciel_suma_3lata_zl"
	ctxKeyOrigPrice         = "cena_originals_zl"
	ctxKeyOrigMargin        = "marza_originals_szt_zl"
	ctxKeyBookPrice         = "cena_ksiazki_zl"
	ctxKeyCatFeePct         = "prowizja_katalog_proc"
	ctxKeyAuthorPct         = "udzial_autora_proc"
	ctxKeyAuthorShare       = "kwota_autora_zl"
	ctxKeyBookFeeUnit       = "prowizja_szt_zl"
	ctxKeyIndiePrice        = "cena_indie_zl"
	ctxKeyProPrice          = "cena_pro_zl"
	ctxKeyUniPrice          = "cena_uczelnia_zl"
	ctxKeyOfficeMonth       = "koszty_biuro_mies_zl"
	ctxKeyTotalBudget       = "budzet_caly_zl"
	ctxKeyTotalBudgetK      = "budzet_caly_tys"
	ctxKeySrcGrants         = "zrodlo_dotacje_zl"
	ctxKeySrcGrantsK        = "zrodlo_dotacje_tys"
	ctxKeySrcInvestors      = "zrodlo_inwestorzy_zl"
	ctxKeySrcInvestorsK     = "zrodlo_inwestorzy_tys"
	ctxKeySrcCrowdfunding   = "zrodlo_crowdfunding_zl"
	ctxKeySrcCrowdfundingK  = "zrodlo_crowdfunding_tys"
	ctxKeySrcOwn            = "zrodlo_wlasne_zl"
	ctxKeySrcOwnK           = "zrodlo_wlasne_tys"
	ctxKeyNonDilutive       = "bezzwrotne_zl"
	ctxKeyNonDilutiveK      = "bezzwrotne_tys"
	ctxKeyNonDilutivePct    = "bezzwrotne_proc"
	ctxKeyInvestAmount      = "kwota_inwestycji_zl"
	ctxKeyInvestAmountK     = "kwota_inwestycji_tys"
	ctxKeyMinTicket         = "bilet_inwestora_min_zl"
	ctxKeyMinTicketPct      = "bilet_inwestora_proc"
	ctxKeyBudPremieres      = "budzet_premiery_zl"
	ctxKeyBudPremieresK     = "budzet_premiery_tys"
	ctxKeyBudIT             = "budzet_it_zl"
	ctxKeyBudITK            = "budzet_it_tys"
	ctxKeyBudRunway         = "budzet_pas_startowy_zl"
	ctxKeyBudDev            = "budzet_rozwoj_zl"
	ctxKeyBudDevK           = "budzet_rozwoj_tys"
	ctxKeyBudSales          = "budzet_sprzedaz_zl"
	ctxKeyBudSalesK         = "budzet_sprzedaz_tys"
	ctxKeyBudPremieresPct   = "budzet_premiery_proc"
	ctxKeyBudDevPct         = "budzet_rozwoj_proc"
	ctxKeyBudSalesPct       = "budzet_sprzedaz_proc"
	ctxKeyCostActors        = "koszt_aktorzy_zl"
	ctxKeyCostSoundStudio   = "koszt_studio_dzwiek_zl"
	ctxKeyCostAuthors       = "koszt_autorzy_zl"
	ctxKeyCostIllustSFX     = "koszt_ilustracje_sfx_zl"
	ctxKeyCostAuditCert     = "koszt_audyt_cert_zl"
	ctxKeyCostMobileDev     = "koszt_mobile_dev_zl"
	ctxKeyCostShopInfra     = "koszt_sklep_infra_zl"
	ctxKeyCostLegal         = "koszt_prawo_umowy_zl"
	ctxKeyCostCampaignCF    = "koszt_kampania_cf_zl"
	ctxKeyCostPromoPrem     = "koszt_promocja_premier_zl"
	ctxKeyCostWorkshopsB2B  = "koszt_warsztaty_b2b_zl"
	ctxKeyCostFairsContests = "koszt_targi_konkursy_zl"
	ctxKeyCostActorsK       = "koszt_aktorzy_tys"
	ctxKeyCostSoundStudioK  = "koszt_studio_dzwiek_tys"
	ctxKeyCostAuthorsK      = "koszt_autorzy_tys"
	ctxKeyCostIllustSFXK    = "koszt_ilustracje_sfx_tys"
	ctxKeyCostMobileDevK    = "koszt_mobile_dev_tys"
	ctxKeyCostStudioDevK    = "koszt_studio_dev_tys"
	ctxKeyRev3Y             = "przychod_3lata_zl"
	ctxKeyProfitR1K         = "zysk_s_r1_tys"
	ctxKeyProfitR3Mln       = "zysk_s_r3_mln"
	ctxKeyEBITDA3Y          = "ebitda_3lata"
	ctxKeyNetProfit3Y       = "nadwyzka_3lata"
	ctxKeyInvProfit3Y       = "zysk_inwestora_3lata_zl"
	ctxKeyCompanyValR3Mln   = "wycena_spolki_r3_mln"
	ctxKeyInvStakeValR3     = "wartosc_udzialow_r3_tys"
	ctxKeyInvROIX           = "zwrot_inwestora_x"
)

type financeYearInput struct {
	rdArchMonthly float64
	bdRetMonthly  float64
	universities  float64
	indieLicenses float64
	proLicenses   float64
	origTitles    float64
	origUnitsPerT float64
	catTitles     float64
	catUnitsPerT  float64
}

type financeConfig struct {
	archEquityPct      float64
	bdEquityPct        float64
	invEquityPct       float64
	archHourlyRate     float64
	bdCommPct          float64
	founderNetMonthly  float64
	founderGrossMonth  float64
	officeMonthly      float64
	citPct             float64
	origPrice          float64
	origMargin         float64
	bookPrice          float64
	catFeePct          float64
	indiePrice         float64
	proPrice           float64
	uniPrice           float64
	varCostPct         float64
	srcGrants          float64
	srcInvestors       float64
	srcCrowdfunding    float64
	srcOwnSales        float64
	minTicket          float64
	multMin            float64
	multMax            float64
	costActors         float64
	costSoundStudio    float64
	costAuthors        float64
	costIllustSFX      float64
	costAuditCert      float64
	costMobileDev      float64
	costShopInfra      float64
	costLegal          float64
	costRunway         float64
	costCampaignCF     float64
	costPromoPremieres float64
	costWorkshopsB2B   float64
	costFairsContests  float64
	years              [3]financeYearInput
}

type financeFieldDesc struct {
	key        string
	defaultVal float64
	ptr        func(*financeConfig) *float64
}

type deckYearCtxKeys struct {
	rdArchMonthly   string
	rdArchYearly    string
	bdRetMonthly    string
	bdOpsYearly     string
	bdAdminYearly   string
	archDivYearly   string
	archTotalYearly string
	archTotalMonth  string
	bdDivYearly     string
	bdTotalYearly   string
	bdTotalMonth    string
	invDivYearly    string
	founderNetMonth string
	universities    string
	indie           string
	pro             string
	origTitles      string
	origUnitsPerT   string
	origTotalUnits  string
	catTitles       string
	catTotalUnits   string
	revOrig         string
	revCat          string
	revLic          string
	revTotal        string
	ebitda          string
	netProfit       string
	months          string
}

var (
	financeConfigSchema = [...]financeFieldDesc{
		{key: "udzial_architekta_proc", defaultVal: 78, ptr: func(c *financeConfig) *float64 { return &c.archEquityPct }},
		{key: "udzial_biznes_proc", defaultVal: 12, ptr: func(c *financeConfig) *float64 { return &c.bdEquityPct }},
		{key: "udzial_inwestorow_proc", defaultVal: 10, ptr: func(c *financeConfig) *float64 { return &c.invEquityPct }},
		{key: "stawka_architekta_godz_zl", defaultVal: 160, ptr: func(c *financeConfig) *float64 { return &c.archHourlyRate }},
		{key: "rd_architekt_r1_mies_zl", defaultVal: 20000, ptr: func(c *financeConfig) *float64 { return &c.years[0].rdArchMonthly }},
		{key: "rd_architekt_r2_mies_zl", defaultVal: 25000, ptr: func(c *financeConfig) *float64 { return &c.years[1].rdArchMonthly }},
		{key: "rd_architekt_r3_mies_zl", defaultVal: 30000, ptr: func(c *financeConfig) *float64 { return &c.years[2].rdArchMonthly }},
		{key: "prowizja_bd_b2b_proc", defaultVal: 15, ptr: func(c *financeConfig) *float64 { return &c.bdCommPct }},
		{key: "bd_retainer_r1_mies_zl", defaultVal: 3000, ptr: func(c *financeConfig) *float64 { return &c.years[0].bdRetMonthly }},
		{key: "bd_retainer_r2_mies_zl", defaultVal: 6000, ptr: func(c *financeConfig) *float64 { return &c.years[1].bdRetMonthly }},
		{key: "bd_retainer_r3_mies_zl", defaultVal: 10000, ptr: func(c *financeConfig) *float64 { return &c.years[2].bdRetMonthly }},

		{key: "wyplata_zalozyciela_netto_mies_zl", defaultVal: 24000, ptr: func(c *financeConfig) *float64 { return &c.founderNetMonthly }},
		{key: "koszt_zalozyciela_brutto_mies_zl", defaultVal: 24000, ptr: func(c *financeConfig) *float64 { return &c.founderGrossMonth }},
		{key: "koszty_biuro_ksiegowosc_mies_zl", defaultVal: 2000, ptr: func(c *financeConfig) *float64 { return &c.officeMonthly }},
		{key: "podatek_cit_efektywny_proc", defaultVal: 15, ptr: func(c *financeConfig) *float64 { return &c.citPct }},

		{key: "cena_originals_zl", defaultVal: 45, ptr: func(c *financeConfig) *float64 { return &c.origPrice }},
		{key: "marza_originals_szt_zl", defaultVal: 32, ptr: func(c *financeConfig) *float64 { return &c.origMargin }},
		{key: "cena_ksiazki_zl", defaultVal: 40, ptr: func(c *financeConfig) *float64 { return &c.bookPrice }},
		{key: "prowizja_katalog_proc", defaultVal: 15, ptr: func(c *financeConfig) *float64 { return &c.catFeePct }},
		{key: "cena_indie_zl", defaultVal: 249, ptr: func(c *financeConfig) *float64 { return &c.indiePrice }},
		{key: "cena_pro_zl", defaultVal: 599, ptr: func(c *financeConfig) *float64 { return &c.proPrice }},
		{key: "cena_uczelnia_zl", defaultVal: 7500, ptr: func(c *financeConfig) *float64 { return &c.uniPrice }},
		{key: "koszt_zmienny_proc", defaultVal: 6, ptr: func(c *financeConfig) *float64 { return &c.varCostPct }},

		{key: "zrodlo_dotacje_zl", defaultVal: 90000, ptr: func(c *financeConfig) *float64 { return &c.srcGrants }},
		{key: "zrodlo_inwestorzy_zl", defaultVal: 80000, ptr: func(c *financeConfig) *float64 { return &c.srcInvestors }},
		{key: "zrodlo_crowdfunding_zl", defaultVal: 70000, ptr: func(c *financeConfig) *float64 { return &c.srcCrowdfunding }},
		{key: "zrodlo_sprzedaz_wlasna_zl", defaultVal: 30000, ptr: func(c *financeConfig) *float64 { return &c.srcOwnSales }},
		{key: "bilet_inwestora_min_zl", defaultVal: 40000, ptr: func(c *financeConfig) *float64 { return &c.minTicket }},
		{key: "mnoznik_wyceny_min", defaultVal: 4.5, ptr: func(c *financeConfig) *float64 { return &c.multMin }},
		{key: "mnoznik_wyceny_max", defaultVal: 6.0, ptr: func(c *financeConfig) *float64 { return &c.multMax }},

		{key: "koszt_aktorzy_zl", defaultVal: 60000, ptr: func(c *financeConfig) *float64 { return &c.costActors }},
		{key: "koszt_studio_dzwiek_zl", defaultVal: 40000, ptr: func(c *financeConfig) *float64 { return &c.costSoundStudio }},
		{key: "koszt_autorzy_zl", defaultVal: 34000, ptr: func(c *financeConfig) *float64 { return &c.costAuthors }},
		{key: "koszt_ilustracje_sfx_zl", defaultVal: 16000, ptr: func(c *financeConfig) *float64 { return &c.costIllustSFX }},

		{key: "koszt_audyt_cert_zl", defaultVal: 18000, ptr: func(c *financeConfig) *float64 { return &c.costAuditCert }},
		{key: "koszt_mobile_dev_zl", defaultVal: 24000, ptr: func(c *financeConfig) *float64 { return &c.costMobileDev }},
		{key: "koszt_sklep_infra_zl", defaultVal: 10000, ptr: func(c *financeConfig) *float64 { return &c.costShopInfra }},
		{key: "koszt_prawo_umowy_zl", defaultVal: 8000, ptr: func(c *financeConfig) *float64 { return &c.costLegal }},
		{key: "koszt_pas_startowy_zl", defaultVal: 0, ptr: func(c *financeConfig) *float64 { return &c.costRunway }},

		{key: "koszt_kampania_cf_zl", defaultVal: 18000, ptr: func(c *financeConfig) *float64 { return &c.costCampaignCF }},
		{key: "koszt_promocja_premier_zl", defaultVal: 24000, ptr: func(c *financeConfig) *float64 { return &c.costPromoPremieres }},
		{key: "koszt_warsztaty_b2b_zl", defaultVal: 10000, ptr: func(c *financeConfig) *float64 { return &c.costWorkshopsB2B }},
		{key: "koszt_targi_konkursy_zl", defaultVal: 8000, ptr: func(c *financeConfig) *float64 { return &c.costFairsContests }},

		{key: "s_r1_uczelnie", defaultVal: 6, ptr: func(c *financeConfig) *float64 { return &c.years[0].universities }},
		{key: "s_r1_indie", defaultVal: 60, ptr: func(c *financeConfig) *float64 { return &c.years[0].indieLicenses }},
		{key: "s_r1_pro", defaultVal: 20, ptr: func(c *financeConfig) *float64 { return &c.years[0].proLicenses }},
		{key: "s_r1_originals_tytuly", defaultVal: 4, ptr: func(c *financeConfig) *float64 { return &c.years[0].origTitles }},
		{key: "s_r1_originals_szt", defaultVal: 2800, ptr: func(c *financeConfig) *float64 { return &c.years[0].origUnitsPerT }},
		{key: "s_r1_katalog_tytuly", defaultVal: 4, ptr: func(c *financeConfig) *float64 { return &c.years[0].catTitles }},
		{key: "s_r1_katalog_szt", defaultVal: 800, ptr: func(c *financeConfig) *float64 { return &c.years[0].catUnitsPerT }},

		{key: "s_r2_uczelnie", defaultVal: 12, ptr: func(c *financeConfig) *float64 { return &c.years[1].universities }},
		{key: "s_r2_indie", defaultVal: 180, ptr: func(c *financeConfig) *float64 { return &c.years[1].indieLicenses }},
		{key: "s_r2_pro", defaultVal: 55, ptr: func(c *financeConfig) *float64 { return &c.years[1].proLicenses }},
		{key: "s_r2_originals_tytuly", defaultVal: 8, ptr: func(c *financeConfig) *float64 { return &c.years[1].origTitles }},
		{key: "s_r2_originals_szt", defaultVal: 2600, ptr: func(c *financeConfig) *float64 { return &c.years[1].origUnitsPerT }},
		{key: "s_r2_katalog_tytuly", defaultVal: 15, ptr: func(c *financeConfig) *float64 { return &c.years[1].catTitles }},
		{key: "s_r2_katalog_szt", defaultVal: 1000, ptr: func(c *financeConfig) *float64 { return &c.years[1].catUnitsPerT }},

		{key: "s_r3_uczelnie", defaultVal: 22, ptr: func(c *financeConfig) *float64 { return &c.years[2].universities }},
		{key: "s_r3_indie", defaultVal: 400, ptr: func(c *financeConfig) *float64 { return &c.years[2].indieLicenses }},
		{key: "s_r3_pro", defaultVal: 110, ptr: func(c *financeConfig) *float64 { return &c.years[2].proLicenses }},
		{key: "s_r3_originals_tytuly", defaultVal: 12, ptr: func(c *financeConfig) *float64 { return &c.years[2].origTitles }},
		{key: "s_r3_originals_szt", defaultVal: 3000, ptr: func(c *financeConfig) *float64 { return &c.years[2].origUnitsPerT }},
		{key: "s_r3_katalog_tytuly", defaultVal: 35, ptr: func(c *financeConfig) *float64 { return &c.years[2].catTitles }},
		{key: "s_r3_katalog_szt", defaultVal: 1200, ptr: func(c *financeConfig) *float64 { return &c.years[2].catUnitsPerT }},
	}

	deckYearVarKeys = [3]deckYearCtxKeys{
		{
			rdArchMonthly:   "rd_arch_r1_mies_zl",
			rdArchYearly:    "rd_arch_r1_rok_zl",
			bdRetMonthly:    "bd_ret_r1_mies_zl",
			bdOpsYearly:     "bd_ops_r1_rok_zl",
			bdAdminYearly:   "bd_admin_r1_rok_zl",
			archDivYearly:   "arch_dyw_r1_zl",
			archTotalYearly: "arch_razem_r1_rok_zl",
			archTotalMonth:  "arch_razem_r1_mies_zl",
			bdDivYearly:     "bd_dyw_r1_zl",
			bdTotalYearly:   "bd_razem_r1_rok_zl",
			bdTotalMonth:    "bd_razem_r1_mies_zl",
			invDivYearly:    "inv_dyw_r1_zl",
			founderNetMonth: "zalozyciel_netto_m_r1",
			universities:    "s_r1_uczelnie",
			indie:           "s_r1_indie",
			pro:             "s_r1_pro",
			origTitles:      "s_r1_orig_tyt",
			origUnitsPerT:   "s_r1_orig_szt",
			origTotalUnits:  "s_r1_orig_suma_szt",
			catTitles:       "s_r1_kat_tyt",
			catTotalUnits:   "s_r1_kat_suma_szt",
			revOrig:         "przychod_orig_r1_zl",
			revCat:          "przychod_kat_r1_zl",
			revLic:          "przychod_lic_r1_zl",
			revTotal:        "przychod_s_r1_zl",
			ebitda:          "ebitda_r1_zl",
			netProfit:       "nadwyzka_r1_zl",
			months:          "months_y1",
		},
		{
			rdArchMonthly:   "rd_arch_r2_mies_zl",
			rdArchYearly:    "rd_arch_r2_rok_zl",
			bdRetMonthly:    "bd_ret_r2_mies_zl",
			bdOpsYearly:     "bd_ops_r2_rok_zl",
			bdAdminYearly:   "bd_admin_r2_rok_zl",
			archDivYearly:   "arch_dyw_r2_zl",
			archTotalYearly: "arch_razem_r2_rok_zl",
			archTotalMonth:  "arch_razem_r2_mies_zl",
			bdDivYearly:     "bd_dyw_r2_zl",
			bdTotalYearly:   "bd_razem_r2_rok_zl",
			bdTotalMonth:    "bd_razem_r2_mies_zl",
			invDivYearly:    "inv_dyw_r2_zl",
			founderNetMonth: "zalozyciel_netto_m_r2",
			universities:    "s_r2_uczelnie",
			indie:           "s_r2_indie",
			pro:             "s_r2_pro",
			origTitles:      "s_r2_orig_tyt",
			origUnitsPerT:   "s_r2_orig_szt",
			origTotalUnits:  "s_r2_orig_suma_szt",
			catTitles:       "s_r2_kat_tyt",
			catTotalUnits:   "s_r2_kat_suma_szt",
			revOrig:         "przychod_orig_r2_zl",
			revCat:          "przychod_kat_r2_zl",
			revLic:          "przychod_lic_r2_zl",
			revTotal:        "przychod_s_r2_zl",
			ebitda:          "ebitda_r2_zl",
			netProfit:       "nadwyzka_r2_zl",
			months:          "months_y2",
		},
		{
			rdArchMonthly:   "rd_arch_r3_mies_zl",
			rdArchYearly:    "rd_arch_r3_rok_zl",
			bdRetMonthly:    "bd_ret_r3_mies_zl",
			bdOpsYearly:     "bd_ops_r3_rok_zl",
			bdAdminYearly:   "bd_admin_r3_rok_zl",
			archDivYearly:   "arch_dyw_r3_zl",
			archTotalYearly: "arch_razem_r3_rok_zl",
			archTotalMonth:  "arch_razem_r3_mies_zl",
			bdDivYearly:     "bd_dyw_r3_zl",
			bdTotalYearly:   "bd_razem_r3_rok_zl",
			bdTotalMonth:    "bd_razem_r3_mies_zl",
			invDivYearly:    "inv_dyw_r3_zl",
			founderNetMonth: "zalozyciel_netto_m_r3",
			universities:    "s_r3_uczelnie",
			indie:           "s_r3_indie",
			pro:             "s_r3_pro",
			origTitles:      "s_r3_orig_tyt",
			origUnitsPerT:   "s_r3_orig_szt",
			origTotalUnits:  "s_r3_orig_suma_szt",
			catTitles:       "s_r3_kat_tyt",
			catTotalUnits:   "s_r3_kat_suma_szt",
			revOrig:         "przychod_orig_r3_zl",
			revCat:          "przychod_kat_r3_zl",
			revLic:          "przychod_lic_r3_zl",
			revTotal:        "przychod_s_r3_zl",
			ebitda:          "ebitda_r3_zl",
			netProfit:       "nadwyzka_r3_zl",
			months:          "months_y3",
		},
	}

	deckMilestones = [36]string{
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
)
