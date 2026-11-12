package book

const (
	ExportFmtPlaybook  = "playbook"
	ExportFmtWASM      = "wasm"
	ExportFmtPDF       = "pdf"
	ExportFmtFountain  = "fountain"
	ExportFmtFDX       = "fdx"
	ExportFmtTreatment = "treatment"
	ExportFmtBookPDF   = "book_pdf"
	ExportFmtVOCSV     = "vo_csv"
	ExportFmtActorPDF  = "actor_pdf"
	ExportFmtPlaytxt   = "playtxt"

	ExportSuffixPlaybook  = ".playbook"
	ExportSuffixWASM      = ".wasm"
	ExportSuffixPDF       = ".pdf"
	ExportSuffixFountain  = ".fountain"
	ExportSuffixFDX       = ".fdx"
	ExportSuffixTreatment = "-treatment.pdf"
	ExportSuffixBookPDF   = "-book.pdf"
	ExportSuffixVOCSV     = "-vo.csv"
	ExportSuffixActorPDF  = "-actors.pdf"
	ExportSuffixPlaytxt   = ".playtxt"
)

var ExportFormats = [...]string{
	ExportFmtPlaybook,
	ExportFmtWASM,
	ExportFmtPDF,
	ExportFmtFountain,
	ExportFmtFDX,
	ExportFmtTreatment,
	ExportFmtBookPDF,
	ExportFmtVOCSV,
	ExportFmtActorPDF,
	ExportFmtPlaytxt,
}

func IsValidExportFormat(id string) bool {
	for _, f := range ExportFormats {
		if f == id {
			return true
		}
	}
	return false
}

