package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportSeasonalFromSourceJSON(t *testing.T) {
	jsonContent := `[
		{
			"primaryFoodId": "test_alcachofa",
			"foodName": "Alcachofa Test",
			"category": "vegetable",
			"peakMonths": [1, 2],
			"availableMonths": [3]
		}
	]`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "seasonal_test.json")
	if err := os.WriteFile(filePath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	items, err := ImportSeasonalFromSource(filePath)
	if err != nil {
		t.Fatalf("expected successful import, got %v", err)
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items (2 peak + 1 available), got %d", len(items))
	}
}

func TestImportSeasonalFromSourceCSV(t *testing.T) {
	csvContent := "food_id,name,category,month,availability,source\nbrocoli_test,Brocoli,vegetable,11,peak,TEST\n"

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "seasonal_test.csv")
	if err := os.WriteFile(filePath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	items, err := ImportSeasonalFromSource(filePath)
	if err != nil {
		t.Fatalf("expected successful import, got %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Month != 11 || items[0].FoodName != "Brocoli" {
		t.Errorf("unexpected item content: %+v", items[0])
	}
}

func TestImportCookingProfilesFromSource(t *testing.T) {
	jsonContent := `[
		{
			"primaryFoodId": "cuscus_test",
			"foodName": "Cuscus Test",
			"category": "cereal",
			"defaultEnergyClass": "minimal",
			"defaultFireMinutes": 1,
			"efficiencyTips": ["Hervir y reposar 5 min"]
		}
	]`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cooking_test.json")
	if err := os.WriteFile(filePath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	profiles, err := ImportCookingProfilesFromSource(filePath)
	if err != nil {
		t.Fatalf("expected successful import, got %v", err)
	}

	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].DefaultFireMinutes != 1 {
		t.Errorf("expected 1 fire minute, got %d", profiles[0].DefaultFireMinutes)
	}
}
