package pruner

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

// InspectBackup analiza el JSON de backup y genera un reporte de inspección con pesos y recomendaciones
func InspectBackup(rawJSON []byte) (*model.BackupInspectionReport, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(rawJSON, &root); err != nil {
		return nil, fmt.Errorf("error parseando JSON de backup: %w", err)
	}

	dataRaw, ok := root["data"]
	if !ok {
		return nil, fmt.Errorf("el backup no contiene la clave 'data'")
	}
	dataMap, ok := dataRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("el contenido de 'data' no es un objeto válido")
	}

	report := &model.BackupInspectionReport{
		TotalSize: int64(len(rawJSON)),
		Tables:    make(map[string]model.TableSummary),
	}

	if v, ok := root["version"].(float64); ok {
		report.Version = int(v)
	}
	if ts, ok := root["timestamp"].(string); ok {
		report.Timestamp = ts
	}

	totalRecs := 0
	var recommendations []string

	for tableName, val := range dataMap {
		list, ok := val.([]interface{})
		if !ok {
			continue
		}
		totalRecs += len(list)

		tableBytes, _ := json.Marshal(list)
		tableSize := int64(len(tableBytes))

		blobCount := 0
		var blobBytes int64 = 0

		for _, itemRaw := range list {
			item, ok := itemRaw.(map[string]interface{})
			if !ok {
				continue
			}
			for k, v := range item {
				kLower := strings.ToLower(k)
				if strings.Contains(kLower, "blob") || strings.Contains(kLower, "imagedata") {
					if strVal, isStr := v.(string); isStr && len(strVal) > 50 {
						blobCount++
						blobBytes += int64(len(strVal))
					} else if mVal, isMap := v.(map[string]interface{}); isMap && len(mVal) > 0 {
						blobCount++
						bJson, _ := json.Marshal(mVal)
						blobBytes += int64(len(bJson))
					}
				}
			}
		}

		report.Tables[tableName] = model.TableSummary{
			Count:     len(list),
			SizeBytes: tableSize,
			BlobCount: blobCount,
			BlobBytes: blobBytes,
		}

		if tableName == "pendingUploads" && blobBytes > 100*1024 {
			recommendations = append(recommendations, fmt.Sprintf("Purgar %d blobs de fotos pendientes/completadas en pendingUploads (Ahorro ~%d KB)", blobCount, blobBytes/1024))
		}
		if tableName == "cartHistory" && blobBytes > 50*1024 {
			recommendations = append(recommendations, fmt.Sprintf("Despojar imágenes de tickets pesados archivados en cartHistory (Ahorro ~%d KB)", blobBytes/1024))
		}
		if tableName == "recentProducts" && len(list) > 20 {
			recommendations = append(recommendations, fmt.Sprintf("Vaciar historial de %d productos recientes consultados en recentProducts", len(list)))
		}
	}

	// Comprobar stock 0 en despensa
	if pantryList, ok := dataMap["pantry"].([]interface{}); ok {
		zeroStock := 0
		for _, itemRaw := range pantryList {
			if item, ok := itemRaw.(map[string]interface{}); ok {
				if amt, ok := item["amount"].(float64); ok && amt <= 0 {
					zeroStock++
				}
			}
		}
		if zeroStock > 0 {
			recommendations = append(recommendations, fmt.Sprintf("Eliminar %d artículos con stock en 0 de la despensa", zeroStock))
		}
	}

	report.TotalRecords = totalRecs
	report.Recommendations = recommendations
	return report, nil
}

// PruneBackup sanea y poda el backup aplicando filtros globales y poda quirúrgica en cascada
func PruneBackup(rawJSON []byte, opts model.PruneOptions) (*model.PruneResult, []byte, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(rawJSON, &root); err != nil {
		return nil, nil, fmt.Errorf("error parseando JSON de backup: %w", err)
	}

	dataRaw, ok := root["data"]
	if !ok {
		return nil, nil, fmt.Errorf("el backup no contiene la clave 'data'")
	}
	dataMap, ok := dataRaw.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("el contenido de 'data' no es un objeto válido")
	}

	origSize := int64(len(rawJSON))
	origRecs := 0
	removedByTable := make(map[string]int)

	for _, v := range dataMap {
		if list, ok := v.([]interface{}); ok {
			origRecs += len(list)
		}
	}

	// Sets para búsqueda rápida de IDs a podar
	delRecipeIDs := make(map[int64]bool)
	for _, id := range opts.DeleteRecipeIDs {
		delRecipeIDs[id] = true
	}
	delDiaryIDs := make(map[int64]bool)
	for _, id := range opts.DeleteDiaryEntryIDs {
		delDiaryIDs[id] = true
	}
	delPantryCodes := make(map[string]bool)
	for _, c := range opts.DeletePantryCodes {
		delPantryCodes[strings.TrimSpace(c)] = true
	}
	delCartHistoryIDs := make(map[int64]bool)
	for _, id := range opts.DeleteCartHistoryIDs {
		delCartHistoryIDs[id] = true
	}
	delShoppingListIDs := make(map[int64]bool)
	for _, id := range opts.DeleteShoppingListIDs {
		delShoppingListIDs[id] = true
	}
	delCustomCodes := make(map[string]bool)
	for _, c := range opts.DeleteCustomProductCodes {
		delCustomCodes[strings.TrimSpace(c)] = true
	}

	// 1. Vaciar recentProducts si se solicita
	if opts.PurgeRecentProducts {
		if list, ok := dataMap["recentProducts"].([]interface{}); ok {
			removedByTable["recentProducts"] = len(list)
			dataMap["recentProducts"] = []interface{}{}
		}
	}

	// 2. Despensa e inventario (Pantry y PantryLog en cascada)
	if pantryList, ok := dataMap["pantry"].([]interface{}); ok {
		var newPantry []interface{}
		removedCodes := make(map[string]bool)

		for _, itemRaw := range pantryList {
			item, ok := itemRaw.(map[string]interface{})
			if !ok {
				continue
			}

			code, _ := item["productCode"].(string)
			code = strings.TrimSpace(code)
			amt, _ := item["amount"].(float64)

			// Verificar si debe eliminarse por stock 0 o selección quirúrgica
			shouldDelete := false
			if opts.PurgeZeroStockPantry && amt <= 0 {
				shouldDelete = true
			}
			if delPantryCodes[code] {
				shouldDelete = true
			}

			if shouldDelete {
				removedCodes[code] = true
				removedByTable["pantry"]++
			} else {
				newPantry = append(newPantry, item)
			}
		}
		dataMap["pantry"] = newPantry

		// Cascada en pantryLog
		if len(removedCodes) > 0 {
			if logList, ok := dataMap["pantryLog"].([]interface{}); ok {
				var newLogs []interface{}
				for _, logRaw := range logList {
					logItem, ok := logRaw.(map[string]interface{})
					if !ok {
						continue
					}
					code, _ := logItem["productCode"].(string)
					code = strings.TrimSpace(code)
					if removedCodes[code] {
						removedByTable["pantryLog"]++
					} else {
						newLogs = append(newLogs, logItem)
					}
				}
				dataMap["pantryLog"] = newLogs
			}
		}
	}

	// 3. Recetas (recipes y recipeVersions en cascada + strip photoBlob)
	if recipeList, ok := dataMap["recipes"].([]interface{}); ok {
		var newRecipes []interface{}
		purgedRecipeIDs := make(map[int64]bool)

		for _, recRaw := range recipeList {
			rec, ok := recRaw.(map[string]interface{})
			if !ok {
				continue
			}
			idFloat, _ := rec["id"].(float64)
			id := int64(idFloat)

			if delRecipeIDs[id] {
				purgedRecipeIDs[id] = true
				removedByTable["recipes"]++
				continue
			}

			if opts.StripRecipePhotos {
				delete(rec, "photoBlob")
				delete(rec, "photoBlobBase64")
				delete(rec, "photoBlobType")
			}
			newRecipes = append(newRecipes, rec)
		}
		dataMap["recipes"] = newRecipes

		// Cascada en recipeVersions
		if len(purgedRecipeIDs) > 0 {
			if rvList, ok := dataMap["recipeVersions"].([]interface{}); ok {
				var newRV []interface{}
				for _, rvRaw := range rvList {
					rv, ok := rvRaw.(map[string]interface{})
					if !ok {
						continue
					}
					recIDFloat, _ := rv["recipeId"].(float64)
					if purgedRecipeIDs[int64(recIDFloat)] {
						removedByTable["recipeVersions"]++
					} else {
						newRV = append(newRV, rv)
					}
				}
				dataMap["recipeVersions"] = newRV
			}
		}
	}

	// 4. Diario (diary y diaryVersions en cascada)
	if diaryList, ok := dataMap["diary"].([]interface{}); ok {
		var newDiary []interface{}
		purgedDiaryIDs := make(map[int64]bool)

		for _, dRaw := range diaryList {
			entry, ok := dRaw.(map[string]interface{})
			if !ok {
				continue
			}
			idFloat, _ := entry["id"].(float64)
			id := int64(idFloat)

			if delDiaryIDs[id] {
				purgedDiaryIDs[id] = true
				removedByTable["diary"]++
				continue
			}
			newDiary = append(newDiary, entry)
		}
		dataMap["diary"] = newDiary

		// Cascada en diaryVersions
		if len(purgedDiaryIDs) > 0 {
			if dvList, ok := dataMap["diaryVersions"].([]interface{}); ok {
				var newDV []interface{}
				for _, dvRaw := range dvList {
					dv, ok := dvRaw.(map[string]interface{})
					if !ok {
						continue
					}
					dIDFloat, _ := dv["diaryEntryId"].(float64)
					if purgedDiaryIDs[int64(dIDFloat)] {
						removedByTable["diaryVersions"]++
					} else {
						newDV = append(newDV, dv)
					}
				}
				dataMap["diaryVersions"] = newDV
			}
		}
	}

	// 5. Historial de compras (cartHistory)
	if chList, ok := dataMap["cartHistory"].([]interface{}); ok {
		var newCH []interface{}
		for _, chRaw := range chList {
			ch, ok := chRaw.(map[string]interface{})
			if !ok {
				continue
			}
			idFloat, _ := ch["id"].(float64)
			id := int64(idFloat)

			if delCartHistoryIDs[id] {
				removedByTable["cartHistory"]++
				continue
			}

			if opts.StripTicketImages {
				delete(ch, "ticketBlob")
				delete(ch, "ticketBlobBase64")
				delete(ch, "ticketBlobType")
				delete(ch, "ticketThumbBlob")
				delete(ch, "ticketThumbBlobBase64")
				delete(ch, "ticketThumbBlobType")
			}
			newCH = append(newCH, ch)
		}
		dataMap["cartHistory"] = newCH
	}

	// 6. Listas de la compra (shoppingLists)
	if slList, ok := dataMap["shoppingLists"].([]interface{}); ok {
		var newSL []interface{}
		for _, slRaw := range slList {
			sl, ok := slRaw.(map[string]interface{})
			if !ok {
				continue
			}
			idFloat, _ := sl["id"].(float64)
			id := int64(idFloat)

			if delShoppingListIDs[id] {
				removedByTable["shoppingLists"]++
				continue
			}
			newSL = append(newSL, sl)
		}
		dataMap["shoppingLists"] = newSL
	}

	// 7. Productos Propios / Personalizados (customProducts)
	if cpList, ok := dataMap["customProducts"].([]interface{}); ok {
		var newCP []interface{}
		for _, cpRaw := range cpList {
			cp, ok := cpRaw.(map[string]interface{})
			if !ok {
				continue
			}
			code, _ := cp["code"].(string)
			code = strings.TrimSpace(code)

			if delCustomCodes[code] {
				removedByTable["customProducts"]++
				continue
			}
			newCP = append(newCP, cp)
		}
		dataMap["customProducts"] = newCP
	}

	// 8. Pending Uploads y fotos
	if puList, ok := dataMap["pendingUploads"].([]interface{}); ok {
		var newPU []interface{}
		for _, puRaw := range puList {
			pu, ok := puRaw.(map[string]interface{})
			if !ok {
				continue
			}
			status, _ := pu["status"].(string)

			if opts.PurgeCompletedUploads && (status == "done" || status == "completed" || status == "uploaded") {
				// Vaciar blobs pesados de subidas ya hechas
				delete(pu, "imageData")
				delete(pu, "imageDataBase64")
				delete(pu, "originalImageData")
				delete(pu, "originalImageDataBase64")
				delete(pu, "blob")
				delete(pu, "blobBase64")
				delete(pu, "thumbnailBlob")
				delete(pu, "thumbnailBlobBase64")
			}
			newPU = append(newPU, pu)
		}
		dataMap["pendingUploads"] = newPU
	}

	// 9. Fotos de platos (mealPhotos)
	if opts.StripMealPhotos {
		if mpList, ok := dataMap["mealPhotos"].([]interface{}); ok {
			removedByTable["mealPhotos"] = len(mpList)
			dataMap["mealPhotos"] = []interface{}{}
		}
	}

	root["data"] = dataMap
	prunedBytes, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("error serializando backup podado: %w", err)
	}

	prunedSize := int64(len(prunedBytes))
	bytesSaved := origSize - prunedSize
	pctSaved := 0.0
	if origSize > 0 {
		pctSaved = math.Max(0, (float64(bytesSaved)/float64(origSize))*100.0)
	}

	prunedRecs := 0
	for _, v := range dataMap {
		if list, ok := v.([]interface{}); ok {
			prunedRecs += len(list)
		}
	}

	result := &model.PruneResult{
		OriginalSizeBytes: origSize,
		PrunedSizeBytes:   prunedSize,
		BytesSaved:        bytesSaved,
		PercentSaved:      pctSaved,
		OriginalRecords:   origRecs,
		PrunedRecords:     prunedRecs,
		RecordsRemoved:    origRecs - prunedRecs,
		RemovedByTable:    removedByTable,
		ModifiedData:      dataMap,
	}

	return result, prunedBytes, nil
}
