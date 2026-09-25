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
