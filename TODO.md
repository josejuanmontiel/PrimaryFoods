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

## 📦 Épica : Hacer analisis de los nombre vacios... 

## 📦 Épica 1: Pre-Tratamiento y Deconstrucción de Productos Comerciales (OFF)

> **Objetivo:** Analizar el texto de ingredientes de los productos de OpenFoodFacts para descomponerlos en sus alimentos primarios base, respetando el orden legal de etiquetado (de mayor a menor proporción).

- [ ] **Parser de Lista de Ingredientes de Mayor a Menor**:
  - Extraer y ordenar los ingredientes de `ingredients_text` de cada producto de OFF.
  - Identificar el **Top 1 a Top 3 de ingredientes base** (ej. en un bote de pisto: 1º Tomate 50%, 2º Calabacín 25%, 3º Pimiento 15%...).
  - Mapear cada término al identificador canónico de `PrimaryFood` (`primary:tomate`, `primary:calabacin`, etc.).
- [ ] **Detector de Aditivos y Grado de Pureza**:
  - Identificar aditivos (E-numbers) y alérgenos en el volcado.
  - Asignar una "métrica de pureza" (relación % alimento primario fresco vs azúcares añadidos, aceites refinados y conservantes).
- [ ] **Buscador Inverso por Bases de Despensa**:
  - Endpoint `/api/products/by-base?primary=garbanzo`: Permite consultar qué productos de la despensa o del supermercado contienen como base un alimento primario concreto.

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
  - [x] Formato JSON unificado `buscatureceta_menu_pack.json`:
    - `recipes`: Recetas seleccionadas del catálogo listas para guardar en Dexie `recipes`.
    - `shoppingList`: Ingredientes faltantes calculados respecto a la despensa para agregar a la compra.
    - `diaryPlan`: Propuesta de planificación de comidas para inyectar en la agenda (`diary`).
  - [x] Botón rápido en tarjetas de recetas `📋 Copiar faltantes (N)` para el portapapeles.
  - [x] Modal global en la UI `🛒 Faltantes y Pack Buscatureceta` con copia de lista de compra y descarga del `.json`.
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
