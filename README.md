# tmzmapper

Una simple utilidad que mapea las zonas horarias de [ActiveSupport](https://api.rubyonrails.org/classes/ActiveSupport/TimeZone.html) con su correspondiente par.

## ¿Por qué?

Debí recibir en una api GO los datos de zona horaria de una aplicación Ruby que usaba TZInfo::Timezone, produciendose problemas al recuperarlas y convertirlas según zona horaria por el mapeo que hace TZInfo.

## Como

tmzmapper intenta cargar un JSON con el mapeo de TZInfo en un `map[string]string`. Si el archivo no existe, está vacío o contiene JSON inválido, descarga la definición de [`TimeZone::MAPPING` de Rails v8.1.4](https://github.com/rails/rails/blob/v8.1.4/activesupport/lib/active_support/values/time_zone.rb), la procesa y reemplaza el caché de forma atómica.

La versión de Rails está fijada para evitar que un cambio incompatible en la rama `main` rompa la aplicación. Para actualizar el mapeo se debe cambiar explícitamente el tag en `rawURL` y ejecutar la suite de pruebas.

## Modo de uso

Instale con `go get`

```go
go get https://github.com/profe-ajedrez/tmzmapper
```

```go

ianaTMZ, err := TZInfoToIANA("Midway Island")
if err != nil {
    panic(err)
}
fmt.Println(ianaTMZ) // "Pacific/Midway"

ianaTMZ, err := TZInfoToIANA("Atlantic Time (Canada)")
if err != nil {
    panic(err)
}
fmt.Println(ianaTMZ) // "America/Halifax"
```


## Aviso

Recuerde, sea civil y consciente. No abuse de la descarga de time_zone.rb. Si lo necesita descargue una vez y cada tanto tiempo actualice
