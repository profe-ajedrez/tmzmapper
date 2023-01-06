# tmzmapper

Una simple utilidad que mapea las zonas horarias de [ActiveSupport](https://api.rubyonrails.org/classes/ActiveSupport/TimeZone.html) con su correspondiente par de la especificación IANA.

## ¿Por qué?

Debí recibir en una api GO los datos de zona horaria de una aplicación Ruby que usaba TZInfo::Timezone, produciendose problemas al recuperarlas y convertirlas según zona horaria por el mapeo que hace TZInfo.

## Como

tmzmapper intenta cargar un json con el mapeo a lo TZInfo en un map[string]string. Si no encuentra dicho json, descarga https://raw.githubusercontent.com/rails/rails/main/activesupport/lib/active_support/values/time_zone.rb y procesa el archivo convirtiendo `TimeZone::MAPPING` en json y guardando el archivo resultante

## Modo de uso

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