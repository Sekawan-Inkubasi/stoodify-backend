package response

import (
	"errors"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Helpers parsing & validasi bersama antar controller.

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Produksi melaporkan apakah aplikasi berjalan di production.
// Diatur lewat APP_ENV (production/prod). Default: development.
func Produksi() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	return env == "production" || env == "prod"
}

// init mendaftarkan nama field JSON pada validator global Gin sedini mungkin.
// Kalau pendaftaran ditunda sampai router dibuat, validator sudah bisa terlanjur
// meng-cache nama field Go (mis. "School") dan error ke client jadi tidak konsisten.
func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		DaftarkanNamaFieldJSON(v)
	}
}

// DaftarkanNamaFieldJSON membuat error validator memakai nama field JSON
// (snake_case) alih-alih nama field Go, supaya client menerima
// `confirm_password` bukan `RegisterInput.ConfirmPassword`.
func DaftarkanNamaFieldJSON(v *validator.Validate) {
	if v == nil {
		return
	}
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		nama := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if nama == "" || nama == "-" {
			return fld.Name
		}
		return nama
	})
}

// ValidationError mengubah error binding/validator menjadi 400 yang aman.
//
//   - Semua environment: `errors` berisi pesan ramah per field (bahasa Indonesia).
//   - Non-production: ditambah `body` berisi error mentah untuk diagnosis.
//   - Production: detail internal (nama struct, tag validator, path file) tidak
//     pernah dikirim ke client — hanya pesan generik + pesan per field.
func ValidationError(c *gin.Context, err error) {
	pesan := PesanValidasi(err)

	if Produksi() {
		if len(pesan) == 0 {
			pesan = map[string]string{"body": "data tidak valid"}
		}
		BadRequest(c, "Data yang dikirim belum lengkap atau tidak valid", pesan)
		return
	}

	errs := map[string]string{"body": err.Error()}
	for k, v := range pesan {
		errs[k] = v
	}
	BadRequest(c, "Body request tidak valid, harap isi formulir dengan benar", errs)
}

// PesanValidasi mengubah validator.ValidationErrors menjadi map field → pesan
// bahasa Indonesia. Mengembalikan nil bila error bukan dari validator.
func PesanValidasi(err error) map[string]string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return nil
	}
	hasil := make(map[string]string, len(ve))
	for _, fe := range ve {
		nama := fe.Field()
		if nama == "" {
			nama = "body"
		}
		if _, sudah := hasil[nama]; sudah {
			continue
		}
		hasil[nama] = pesanTag(fe)
	}
	return hasil
}

// pesanTag menerjemahkan tag validator menjadi kalimat yang bisa ditampilkan
// langsung di aplikasi.
func pesanTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required", "required_if", "required_with", "required_without":
		return "wajib diisi"
	case "email":
		return "format email tidak valid"
	case "url":
		return "format URL tidak valid"
	case "uuid", "uuid4":
		return "format ID tidak valid"
	case "numeric", "number":
		return "harus berupa angka"
	case "boolean":
		return "harus bernilai true/false"
	case "min":
		if fe.Kind() == reflect.String {
			return "minimal " + fe.Param() + " karakter"
		}
		return "minimal " + fe.Param()
	case "max":
		if fe.Kind() == reflect.String {
			return "maksimal " + fe.Param() + " karakter"
		}
		return "maksimal " + fe.Param()
	case "len":
		return "harus tepat " + fe.Param() + " karakter"
	case "gte":
		return "minimal " + fe.Param()
	case "lte":
		return "maksimal " + fe.Param()
	case "gt":
		return "harus lebih besar dari " + fe.Param()
	case "lt":
		return "harus lebih kecil dari " + fe.Param()
	case "oneof":
		return "harus salah satu dari: " + strings.ReplaceAll(fe.Param(), " ", ", ")
	case "datetime":
		return "format tanggal/waktu tidak valid (contoh: " + fe.Param() + ")"
	default:
		return "tidak valid"
	}
}

// ServerError mencatat detail error di log server, tetapi client hanya menerima
// pesan generik (tidak pernah membocorkan error SQL/internal).
func ServerError(c *gin.Context, err error) {
	metode, path := "", ""
	if c.Request != nil {
		metode, path = c.Request.Method, c.Request.URL.Path
	}
	log.Printf("ERROR %s %s: %v", metode, path, err)
	Fail(c, http.StatusInternalServerError, "mohon maaf ada kesalahan di sisi kami", nil)
}
