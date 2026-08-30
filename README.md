# 🥦 PrimaryFoods

Servicio y base de datos suplementaria a **OpenFoodFacts** para catalogar **alimentos primarios y frescos**, sus valores nutricionales estándar (estilo BEDCA / USDA), micronutrientes, compuestos bioactivos, **bondades para la salud** y sinergias culinarias.

Desarrollado en **Go** con **SQLite embebido** (puro en Go, sin CGO) para máxima velocidad, portabilidad y bajo consumo de recursos.

---

## 🚀 Inicio Rápido

```bash
# 1. Ejecutar el servidor (puerto 8086 por defecto)
make run

# 2. (Opcional) Importar el dataset oficial completo de BEDCA (~1.000 alimentos):
make import-bedca
```

---

## 📡 Endpoints API

| Método | Endpoint | Descripción |
|---|---|---|
| `GET` | `/api/health` | Estado del servicio |
| `GET` | `/api/foods` | Listar catálogo (parámetros: `category`, `tag`, `limit`, `offset`) |
| `GET` | `/api/foods/search?q=merluza` | Búsqueda inteligente por nombre o sinónimos (Smart Match) |
| `GET` | `/api/foods/{id}` | Detalle completo de un alimento y sus bondades |
| `POST` | `/api/foods` | Insertar o actualizar un alimento primario |
| `GET` | `/api/export/json` | Exportar todo el catálogo a JSON compacto para uso offline en **Buscatureceta** |

---

## 📥 Importadores de Datasets Oficiales

### BEDCA (Base de Datos Española de Composición de Alimentos)
Importa automáticamente los **957 alimentos oficiales** con sus macros, vitaminas, minerales y sinónimos en español:

```bash
go run ./cmd/importer -db data/primaryfoods.db
```

---

## 🧪 Tests

```bash
make test
```
