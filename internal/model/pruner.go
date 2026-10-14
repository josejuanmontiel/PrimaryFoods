package model

// TableSummary representa las estadísticas de una tabla en un backup de NutriAgenda
type TableSummary struct {
	Count      int     `json:"count"`
	SizeBytes  int64   `json:"sizeBytes"`
	BlobCount  int     `json:"blobCount"`
	BlobBytes  int64   `json:"blobBytes"`
}

// BackupInspectionReport contiene el desglose detallado de un backup antes de podar
type BackupInspectionReport struct {
	Version      int                     `json:"version"`
	Timestamp    string                  `json:"timestamp"`
	TotalSize    int64                   `json:"totalSize"`
	TotalRecords int                     `json:"totalRecords"`
	Tables       map[string]TableSummary `json:"tables"`
	Recommendations []string             `json:"recommendations"`
}

// PruneOptions define los filtros automáticos y las selecciones manuales para el saneamiento
type PruneOptions struct {
	// Filtros Rápidos (1-Clic)
	PurgeRecentProducts  bool `json:"purgeRecentProducts"`  // Vacía tabla recentProducts
	PurgeZeroStockPantry bool `json:"purgeZeroStockPantry"` // Elimina artículos con amount <= 0 y sus pantryLogs
	PurgeCompletedUploads bool `json:"purgeCompletedUploads"` // Elimina blobs de uploads con status 'done'
	StripTicketImages    bool `json:"stripTicketImages"`    // Quita ticketBlob y ticketThumbBlob de cartHistory manteniendo datos
	StripMealPhotos      bool `json:"stripMealPhotos"`      // Quita blobs de mealPhotos
	StripRecipePhotos    bool `json:"stripRecipePhotos"`    // Quita photoBlob de recetas

	// Poda Quirúrgica / Cascada
	DeleteRecipeIDs      []int64  `json:"deleteRecipeIds"`      // IDs de recetas a eliminar (purga en cascada recipeVersions)
	DeleteDiaryEntryIDs  []int64  `json:"deleteDiaryEntryIds"`  // IDs de comidas de diario a eliminar (purga en cascada diaryVersions)
	DeletePantryCodes    []string `json:"deletePantryCodes"`    // productCodes a eliminar de despensa (purga en cascada pantryLog)
	DeleteCartHistoryIDs []int64  `json:"deleteCartHistoryIds"` // IDs de compras archivadas a eliminar
	DeleteShoppingListIDs []int64 `json:"deleteShoppingListIds"` // IDs de listas de compra a eliminar
	DeleteCustomProductCodes []string `json:"deleteCustomProductCodes"` // Códigos de productos manuales a eliminar
}

// PruneResult contiene las métricas del resultado del proceso de poda
type PruneResult struct {
	OriginalSizeBytes int64                  `json:"originalSizeBytes"`
	PrunedSizeBytes   int64                  `json:"prunedSizeBytes"`
	BytesSaved        int64                  `json:"bytesSaved"`
	PercentSaved      float64                `json:"percentSaved"`
	OriginalRecords   int                    `json:"originalRecords"`
	PrunedRecords     int                    `json:"prunedRecords"`
	RecordsRemoved    int                    `json:"recordsRemoved"`
	RemovedByTable    map[string]int         `json:"removedByTable"`
	ModifiedData      map[string]interface{} `json:"-"`
}
