package pruner_test

import (
	"os"
	"testing"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
	"github.com/josejuanmontiel/primaryfoods/internal/pruner"
)

func TestInspectAndPruneBackup(t *testing.T) {
	backupPath := "../../nutriagenda_backup_2026-09-28.json"
	raw, err := os.ReadFile(backupPath)
	if err != nil {
		t.Skipf("Fichero de backup no encontrado en %s: %v", backupPath, err)
		return
	}

	report, err := pruner.InspectBackup(raw)
	if err != nil {
		t.Fatalf("InspectBackup falló: %v", err)
	}

	if report.Version != 1 {
		t.Errorf("Esperaba version 1, obtenido %d", report.Version)
	}
	if len(report.Tables) == 0 {
		t.Errorf("No se encontraron tablas en el backup")
	}
	if report.TotalRecords <= 0 {
		t.Errorf("Esperaba totalRecords > 0, obtenido %d", report.TotalRecords)
	}

	// Probar poda rápida
	opts := model.PruneOptions{
		PurgeRecentProducts:   true,
		PurgeZeroStockPantry:  true,
		PurgeCompletedUploads: true,
		StripTicketImages:     true,
	}

	res, prunedBytes, err := pruner.PruneBackup(raw, opts)
	if err != nil {
		t.Fatalf("PruneBackup falló: %v", err)
	}

	if res.BytesSaved <= 0 {
		t.Errorf("Esperaba bytesSaved > 0, obtenido %d", res.BytesSaved)
	}
	if res.PercentSaved < 50.0 {
		t.Errorf("Esperaba al menos 50%% de ahorro con poda de blobs, obtenido %.2f%%", res.PercentSaved)
	}

	// Inspeccionar el podado para comprobar que sigue siendo válido
	prunedReport, err := pruner.InspectBackup(prunedBytes)
	if err != nil {
		t.Fatalf("InspectBackup del podado falló: %v", err)
	}
	if prunedReport.Tables["recentProducts"].Count != 0 {
		t.Errorf("recentProducts debería estar vacío, tiene %d", prunedReport.Tables["recentProducts"].Count)
	}
}

func TestCascadingDeletes(t *testing.T) {
	miniBackup := []byte(`{
		"version": 1,
		"timestamp": "2026-10-14T10:00:00Z",
		"data": {
			"recipes": [
				{"id": 1, "name": "Paella"},
				{"id": 2, "name": "Tortilla"}
			],
			"recipeVersions": [
				{"recipeId": 1, "version": 1},
				{"recipeId": 1, "version": 2},
				{"recipeId": 2, "version": 1}
			],
			"pantry": [
				{"productCode": "P1", "amount": 100},
				{"productCode": "P2", "amount": 0}
			],
			"pantryLog": [
				{"productCode": "P1", "delta": 100},
				{"productCode": "P2", "delta": 50}
			],
			"diary": [
				{"id": 10, "mealType": "lunch"}
			],
			"diaryVersions": [
				{"diaryEntryId": 10, "action": "consumed"}
			]
		}
	}`)

	opts := model.PruneOptions{
		DeleteRecipeIDs:      []int64{1},
		PurgeZeroStockPantry: true,
		DeleteDiaryEntryIDs:  []int64{10},
	}

	res, _, err := pruner.PruneBackup(miniBackup, opts)
	if err != nil {
		t.Fatalf("PruneBackup falló: %v", err)
	}

	if res.RemovedByTable["recipes"] != 1 {
		t.Errorf("Esperaba 1 receta eliminada, obtenido %d", res.RemovedByTable["recipes"])
	}
	if res.RemovedByTable["recipeVersions"] != 2 {
		t.Errorf("Esperaba 2 recipeVersions eliminadas en cascada, obtenido %d", res.RemovedByTable["recipeVersions"])
	}
	if res.RemovedByTable["pantry"] != 1 {
		t.Errorf("Esperaba 1 pantry eliminado por stock 0, obtenido %d", res.RemovedByTable["pantry"])
	}
	if res.RemovedByTable["pantryLog"] != 1 {
		t.Errorf("Esperaba 1 pantryLog eliminado en cascada, obtenido %d", res.RemovedByTable["pantryLog"])
	}
	if res.RemovedByTable["diary"] != 1 {
		t.Errorf("Esperaba 1 diary eliminado, obtenido %d", res.RemovedByTable["diary"])
	}
	if res.RemovedByTable["diaryVersions"] != 1 {
		t.Errorf("Esperaba 1 diaryVersions eliminado en cascada, obtenido %d", res.RemovedByTable["diaryVersions"])
	}
}
