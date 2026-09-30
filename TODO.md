# 📋 Roadmap & TODO — PrimaryFoods (Motor Analítico & Planificador)

PrimaryFoods evoluciona para convertirse en el **cerebro nutricional, analizador previo y generador de menús** que conecta el flujo de captura (**Mealie**), la base de datos comercial (**OpenFoodFacts**) y la gestión offline de despensa, diario y compras (**Buscatureceta / NutriAgenda**).

---

## 🎯 Visión del Flujo Integral ("Despensa → Receta → Compra → Diario → Ajuste Real")

```
[OpenFoodFacts Dump]
       │
       ▼ (Pre-tratamiento: Parser ingredientes de > a < peso)
[Productos deconstruidos en "Bases"] ──┐
                                      │
[Stock Despensa Buscatureceta] ───────┼──► [Motor de Ideas & Búsqueda Mealie]
                                      │         │
[Catálogo Alimentos Primarios (BEDCA)]│         ▼
[Calendario Estacional (52 Semanas)] ─┴──► [Generador de Menú Semanal Inteligente]
                                                │
                                                ▼ (Export JSON)
                                     [NutriAgenda / Buscatureceta]
                                     ├── 1. Plan Semanal en Agenda
                                     └── 2. Carrito (Menú - Despensa)
                                                │
                                                ▼ (Flujo Diario)
                                     [Reconciliación: Plan vs Realidad]
                                                │
                                                ▼ (Feedback Loop)
                                     [Cálculo de Carencias para Semana N+1]
```

---

## 📦 Épica 1: Pre-Tratamiento, Ingestión y Deconstrucción de Productos Comerciales (OFF)

> **Objetivo:** Analizar el texto de ingredientes de los productos de OpenFoodFacts para descomponerlos en sus alimentos primarios base, vincular recetas con opciones comerciales reales y clasificar por calidad nutricional (Nutriscore, Nova, pureza).

- [x] **Ingestión e Indexación Ligera de OpenFoodFacts (España)**:
  - [x] Parser en streaming (`internal/importer/off.go`) para `spain_products.tsv.zz` (zlib).
  - [x] Tabla SQLite `commercial_products` con más de 50.000 productos con ingredientes, Nutriscore (A-E), Nova (1-4) y macros.
  - [x] Buscador de productos comerciales (`/api/products/search`) ordenado por salud/calidad (Nutriscore A/B y Nova 1/2 primero).
- [x] **Ingestor de Recetas JSON / Schema.org (Paso 0 - OCR Qwen-VL & Mealie)**:
  - [x] Parser `internal/importer/recipe_json.go` compatible con extracciones OCR de PDFs y Schema.org.
  - [x] Endpoint `POST /api/recipes/import-json` y flag CLI `-recipes` para carga aditiva.
- [x] **Analizador y Deconstructor de Ingredientes de Recetas ("Despensa / Supermercado")**:
  - [x] Algoritmo `internal/matcher/analyzer.go` que descompone cada ingrediente:
    1. Limpieza de medidas/unidades (`CleanIngredientName`: tazas, cucharadas, tiras, stopwords).
    2. Identificación de Alimento Primario base (`BEDCA` + aliases con límites de palabra para evitar falsos positivos).
    3. Búsqueda de alternativas comerciales de supermercado en OFF (`commercial_products`) clasificadas por calidad.
    4. Detección y reporte de ingredientes huérfanos.
  - [x] CLI de benchmarking `cmd/analyzer/main.go` y endpoint `GET /api/recipes/analyze`.
  - [x] **Resultado benchmark 15 recetas extraídas de PDF**: 100% de cobertura resuelta (80% primarios, 20% productos comerciales OFF, 0% huérfanos).
- [x] **Detector de Aditivos y Grado de Pureza Normativo (Reglamento UE 1169/2011)**:
  - [x] Parser `internal/importer/label_parser.go` que descompone `ingredients_text` respetando orden legal decreciente por peso.
  - [x] Extracción de porcentajes QUID (`25%`, `55,5%`).
  - [x] Detector de aditivos (E-numbers y categorías funcionales) con clasificación de riesgo (`alto`, `medio`, `bajo`).
  - [x] Detector de azúcares añadidos/jarabes y grasas refinadas de palma.
  - [x] Cálculo de `EstimatedPurityScore` (0-100) y deconstrucción de productos (`GET /api/products/{code}/deconstruct`).
- [x] **Buscador Inverso por Bases de Despensa**:
  - [x] Endpoint `GET /api/products/by-base?primary=garbanzo` y CLI (`-by-base garbanzo`).
  - [x] Localiza qué productos del supermercado contienen como base principal o secundaria un alimento primario concreto, ordenados por pureza.

---

## 🍳 Épica 2: Indexación Masiva de Mealie & Grafo de Recetas

> **Objetivo:** Conectar PrimaryFoods con Mealie para pre-analizar todo el recetario del usuario y crear un grafo de recetas ↔ ingredientes primarios.

- [x] **Conector API Mealie en Go**:
  - [x] Sincronización batch con Mealie (`GET /api/recipes`) o importación desde JSON local (`internal/mealie`).
  - [x] Cacheo local en SQLite de recetas (`mealie_recipes`), pasos, raciones e ingredientes.
- [x] **Normalizador de Ingredientes y Motor "Despensa ➔ Receta"**:
  - [x] Algoritmo de Smart Matching semántico en cascada cruzando despensa con recetas de Mealie (`internal/matcher`).
  - [x] Enlazar cada ingrediente textual con su `PrimaryFood` correspondiente y sinónimos (BEDCA Aliases).
  - [x] Endpoint `/api/recipes/matching` con clasificación de recetas 100% listas para cocinar vs faltantes.
- [x] **Identificador de Recetas Faltantes**:
  - [x] Desglose de `MissingIngredients` y `MissingCount` por receta para nutrir la lista de la compra de Buscatureceta.

---

## 🗓️ Épica 3: Calendario Estacional y Generador de Menús (52 Semanas)

> **Objetivo:** Generar menús semanales completos y equilibrados según la semana del año, disponibilidad estacional y existencias de despensa.

- [ ] **Matriz de Estacionalidad (52 semanas en España)**:
  - Definir meses/semanas óptimas para cada alimento primario (verduras, frutas, pescados de temporada).
- [ ] **Motor de Reglas Nutricionales y Rotación**:
  - Criterios mediterráneos configurables:
    - 3–4 días de legumbres a la semana.
    - 2–3 días de pescado (al menos 1 azul).
    - Variedad cromática/polifenoles diaria y fuentes de fibra.
- [ ] **Generador de Menú Semanal Adaptativo**:
  - Algoritmo que selecciona recetas del catálogo indexado de Mealie que cumplen las reglas de la semana y priorizan lo que ya existe en la despensa.
  - Endpoint `/api/menu/generate?week=XX&pantry=[codes...]`.

---

## 🛒 Épica 4: Exportación y Cierre de Brecha con Buscatureceta

> **Objetivo:** Descargar el plan semanal con un clic e inyectarlo en Buscatureceta sin fricción manual.

- [x] **Ingestión de Paquete Modular de Buscatureceta**:
  - [x] Endpoint `POST /api/sync/buscatureceta` para recibir `{ pantry, diary, diaryVersions }`.
  - [x] Almacenamiento en SQLite de `user_pantry` y `user_diary_history`.
- [x] **Esquema de Exportación Compatible y Gestor de Faltantes**:
  - [x] Formato JSON unificado `buscatureceta_menu_pack.json` y exportador `/api/export/buscatureceta`:
    - `recipes`: Recetas analizadas con vinculación canónica BEDCA (`primaryFoodId`) y alternativas comerciales de supermercados españoles (EAN, Nutriscore, pureza).
    - `shoppingList`: Ingredientes faltantes calculados respecto a la despensa para agregar a la compra.
    - `diaryPlan`: Propuesta de planificación de comidas para inyectar en la agenda (`diary`).
  - [x] Botón rápido en tarjetas de recetas `📋 Copiar faltantes (N)` para el portapapeles.
  - [x] Modal global en la UI `🛒 Faltantes y Pack Buscatureceta` con copia de lista de compra y descarga del `.json`.
  - [x] Especificación OpenAPI 3.1.0 completa (`/api/openapi.yaml`, `/api/openapi.json`) y visor interactivo `/docs` (Scalar UI).
  - [x] Web SPA analítica unificada con 6 pestañas interactivas (Analizador Recetas, Buscador Base OFF, Deconstructor Legal UE 1169/2011, Despensa ➔ Receta, Exportador Buscatureceta, OpenAPI 3.1).
- [ ] **Importador Aditivo en Buscatureceta (Próxima Sesión)**:
  - [ ] Implementar `importMenuPlanPackage(json)` en Buscatureceta para realizar una importación aditiva/merge (sin limpiar la base de datos).
  - [ ] Inyectar recetas en `db.recipes`, comidas en `db.diary` y faltantes en `db.cart`.
  - [ ] Botón en Buscatureceta: *"📥 Importar Plan de Menú / PrimaryFoods"*.

---

## 🔄 Épica 5: Reconciliación Diaria (Planificado vs Real) y Feedback Loop

> **Objetivo:** Transformar la agenda estática en un sistema de aprendizaje continuo mediante el ajuste de lo que realmente se come.

- [x] **Modo "Reconciliación del Día" en el Diario de Buscatureceta**:
  - [x] Confirmar que comió lo planificado (1-click con Quick Check-In).
  - [x] Sustituir o desmarcar platos omitidos (aperitivo/postre).
  - [x] Historial de decisiones inmutable en `diaryVersions`.
- [x] **Descuento de Despensa Reconciliado**:
  - [x] El stock de despensa se descuenta según lo **realmente consumido**, evitando desfases en el inventario con toggle de cocinado en casa.
- [ ] **Detector de Carencias y Retroalimentación**:
  - Resumen semanal de ingestas reales: *"Esta semana faltó un 30% del objetivo de fibra y no hubo ración de pescado azul"*.
  - Enviar estas carencias como parámetro al generador del siguiente menú semanal en PrimaryFoods para compensarlas automáticamente.

---

## ✂️ Épica 6: Visualizador & Podador de Backups de NutriAgenda (Curación & Saneamiento)

> **Objetivo:** Disponer de una estación visual en PrimaryFoods con interfaz simplificada estilo NutriAgenda para inspeccionar copias de seguridad (`nutriagenda_backup_*.json`), "podar" de forma selectiva o masiva recetas, despensa, diario y compras obsoletas, retocar inventario a gusto y purgar blobs innecesarios, exportando un JSON limpio, íntegro y curado para NutriAgenda.

- [ ] **Arquitectura de Poda en Cascada e Integridad Referencial**:
  - `recipes` ➔ Purgar automáticamente sus snapshots en `recipeVersions` y advertir si existen referencias vivas en `diary`.
  - `pantry` ➔ Ofrecer purga sincronizada en `pantryLog` al eliminar un producto.
  - `diary` ➔ Purgar en cascada sus entradas históricas en `diaryVersions`.
- [ ] **Doble Modo de Poda en la UI**:
  - **1. Modo Poda Quirúrgica (Manual por secciones)**:
    - **🍳 Recetas & Plantillas (`recipes`, `recipeVersions`, `mealTemplates`)**: Grid de tarjetas con ingredientes desplegables y botón `🗑️ Podar`.
    - **🥫 Despensa e Inventario (`pantry`, `pantryLog`)**: Selector de zona (`food` / `nonfood`), buscador instantáneo, edición ágil inline de cantidades/unidades y botón `❌ Podar`.
    - **📅 Diario y Objetivos (`diary`, `diaryVersions`, `goals`)**: Línea temporal de comidas para podar días completos o tomas individuales de prueba.
    - **🛒 Compras y Listas (`shoppingLists`, `cart`, `cartHistory`, `priceHistory`)**: Eliminación de listas archivadas, carritos y tickets antiguos.
    - **🏷️ Productos Propios & Frecuentes (`customProducts`, `recentProducts`)**: Poda de productos manuales obsoletos y vaciado del historial de escaneos.
    - **📸 Multimedia y Subidas (`pendingUploads`, `mealPhotos`)**: Purgado de fotos subidas y fotos de platos para aligerar espacio.
  - **2. Modo Poda Rápida en 1 Clic (Filtros Globales de Saneamiento)**:
    - `[ 🧹 Vaciar productos recientes / caché ]`
    - `[ 🧹 Eliminar artículos con stock 0 de despensa ]`
    - `[ 🧹 Purgar blobs de uploads completados a OFF (-900 KB) ]`
    - `[ 🧹 Despojar fotos pesadas de tickets archivados conservando metadatos ]`
- [ ] **Métricas Reactivas y Exportación NutriAgenda**:
  - Contador reactivo de poda (ej: *"Recetas: 5 ➔ 3 \| Despensa: 51 ➔ 42 \| Tamaño: 1.34 MB ➔ 38 KB"*).
  - Botón: *"📥 Descargar Backup Podado"* (JSON 100% compatible con el importador de NutriAgenda / Buscatureceta).


---

## 📅 Épica 7: Planificación de Menús a Largo Plazo (Fase 1 → 4)

> **Objetivo:** Sistema completo de planificación de menús anuales basado en vida útil, estacionalidad y criterios mediterráneos.

### ✅ Fase 1 — Vida Útil y Fechas de Caducidad (COMPLETADA)

- [x] **Dexie v14**: Índice `expiryDate` en tabla `pantry` (Buscatureceta)
- [x] **ShelfLifeEstimator.js**: Motor heurístico 3 capas (OFF categories_tags → BEDCA → palabras clave) con fallback
- [x] **PantryStore.js**: `addStock()` con estimación automática; `updateExpiryDate()`; `getPantryInventory()` enriquecido con `expiryInfo`
- [x] **UI**: Badge de caducidad coloreado en tarjetas (🟢 larga / 🟡 media / 🔴 crítica); editor en modal detalle; campo fecha en modal "Añadir Stock"
- [x] **PrimaryFoods model**: `PantryItem` + `ExpiryDate`, `OpenedAt`, `IsEstimated`, `ShelfClass`
- [x] **`internal/shelflife`**: Paquete Go con misma lógica de estimación que el motor JS
- [x] **SQLite migration**: Columnas aditivas idempotentes en `user_pantry`; `GetPantryItems()` ordenado por caducidad
- [x] **API**: `GET /api/pantry/shelf-life/estimate?code=&name=&categories_tags=&primary_category=`
- [x] **Tests**: Playwright E2E (Buscatureceta) + 5 tests unitarios Go (PrimaryFoods)

### ✅ Fase 2 — Matriz de Estacionalidad 52 Semanas (COMPLETADA)

- [x] **Tabla `seasonal_calendar`** en PrimaryFoods SQLite (`primary_food_id`, `food_name`, `category`, `month`, `availability`, `source`)
- [x] **Seed estacional** con datos reales de España (`internal/seed/seasonal.json`):
  - Verduras (alcachofa, brócoli, espinacas, calabaza, tomate)
  - Frutas (fresa/fresón, cereza, naranja, uva blanca)
  - Pescados azules y blancos (sardina, bonito del norte, caballa)
- [x] **Endpoint** `GET /api/seasonal/calendar?month=X&week=Y&category=Z&availability=W`
- [x] **Lógica de semana/mes**: soporte de parámetro `week` (1-52) con cálculo de mes de referencia
- [x] **Tests**: Cobertura con test unitario en Go (`seasonal_test.go`)

### ✅ Fase 3 — Motor de Reglas y Generador de Menú Semanal (COMPLETADA)

- [x] **Modelo `WeeklyMenuPlan` y `DailyMealSlot`** (`internal/model/menu_plan.go`):
  - 14 slots (7 almuerzos + 7 cenas), contadores de legumbres, pescado, pescado azul, estacionales y rescates de despensa.
- [x] **Motor de reglas mediterráneas** (`internal/planner/planner.go`):
  - Detección de legumbres (lentejas, garbanzos, alubias) y pescados (blanco y azul como salmón, sardina, bonito, caballa).
  - Distribución inteligente (almuerzos más energéticos, cenas más ligeras).
  - Control de variedad: penalización de repetición en la misma semana.
- [x] **Priorización de despensa anti-desperdicio**:
  - Bonus para ingredientes marcados como `critical` o `short` (< 15 días a caducar).
- [x] **Alineación con estacionalidad**:
  - Bonus para alimentos en temporada pico (`peak`) o disponible (`available`).
- [x] **Endpoints API**:
  - `GET /api/menu/generate?week=XX&year=YYYY&prioritize_pantry=true&fuel_saving=true`
  - `POST /api/menu/generate` (para planificación ad-hoc con recetas o despensa personalizadas)
  - `GET /api/cooking/energy-profiles?class=` (zero, minimal, medium, high)
- [x] **Eficiencia Energética de Cocinado ("Low-Fuel / Survival Mode")**:
  - Clasificación en 4 niveles de combustible (`zero`, `minimal`, `medium`, `high`) y catálogo semilla (`cooking_energy.json`).
  - Métodos pasivos de ahorro (cuscús en reposo térmico, cocción pasiva de pasta/huevos con tapa, remojo de legumbres y alternativa directa de conserva).
  - Modo `fuelSavingMode` en el planificador: prioriza platos sin fuego (`zero`) y de hervor rápido (`minimal`), penalizando cocciones largas.
- [x] **Ampliación Catálogo Estacional y Eficiencia Energética**:
  - Formato compacto `seasonal_definitions.json` con más de 66 alimentos españoles (MAPA) expandidos a 479 registros anuales.
  - Catálogo extendido `cooking_energy.json` con 25 perfiles de preparación (zero, minimal, medium, high), técnicas pasivas térmicas y alternativas de conserva.
- [x] **Importadores de Fuentes Externas (Remotas / Locales)**:
  - `internal/importer/seasonal.go`: Importador polimórfico de calendarios estacionales (JSON directo, definiciones compactas `peakMonths`/`availableMonths`, o CSV) desde URL HTTP o fichero local.
  - `internal/importer/cooking.go`: Importador de perfiles de preparación y eficiencia energética desde URL o fichero (compatible con formatos directos y USDA FoodKeeper).
  - Endpoints de importación: `POST /api/seasonal/import` y `POST /api/cooking/import`.
  - Cobertura con test unitario en `seasonal_cooking_test.go`.
- [x] **Tests**: Cobertura con test unitario en Go (`planner_test.go`, `seasonal_test.go`, `menu_test.go` y `seasonal_cooking_test.go`)

### ✅ Fase 4 — Planificador Anual Batch y Exportación (COMPLETADA)

- [x] **Algoritmo batch 52 semanas** (sin repetir receta > 1x cada 10 días)
- [x] **UI en PrimaryFoods**: vista calendario anual (grid semanas, agrupado por mes, expandible)
- [x] **Export masivo JSON** → importar en NutriAgenda / Buscatureceta (`/api/menu/annual/export`)
