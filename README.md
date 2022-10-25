# Maps и утечки памяти в GO.

CreatedAt: October 20, 2022 9:35 PM
EditAt: October 21, 2022 9:17 AM
Favorite: No
Tags: DataStruct, GarbageCollector, Golang
Type: Code
URL: https://teivah.medium.com/maps-and-memory-leaks-in-go-a85ebe6e7e69

# Maps и утечки памяти в GO.

## Введение:

Map в go исключительно растут и никогда не высвобождаются память(правильный термин shrink). О подобном поведении мало кто знает, но оно ведет к утечкам памяти о которых важно помнить.

При работе с map важно понимать то как они растут и освобождают память. Давайте попробуем понять это детальнее.

## Наглядный пример:

Что бы понять лучше как работают maps в go, давайте приведем наглядный пример.

Создадим специальную функцию, которая будет печать количество используемой (аллоцированой) памяти.

```go
func printAlloc() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("%d mb\n", m.Alloc/(1024*1024))
}
```

После этого напишем небольшой кусок кода, по работе с maps.

```go
func main() {
	n := 20_000_000
	m := make(map[int][128]byte)
	printAlloc()

	for i := 0; i < n; i++ {
		m[i] = [128]byte{}
	}
	printAlloc()

	for i := 0; i < n; i++ { 
		delete(m, i)
	}

	runtime.GC() 
	printAlloc()
	runtime.KeepAlive(m) // это нужно что бы GC не собрал нашу map
}
```

Алгоритм действий следующий:

1. Создаем пустую map
2. Заполним ее 20_000_000 объектов
3. Выведем количество памяти
4. Удалим все элементы
5. Превентивно вызовем garbage collector
6. Выведем количество памяти

Выведем результат этой программы:

```bash
0 mb <-- После создания пустой map
7389 mb <-- После добавления 20 миллионов элементов
4694 mb <-- После удаления элементов и GC
```

![Снимок экрана 2022-10-20 в 22.05.37.png](Maps%20%D0%B8%20%D1%83%D1%82%D0%B5%D1%87%D0%BA%D0%B8%20%D0%BF%D0%B0%D0%BC%D1%8F%D1%82%D0%B8%20%D0%B2%20GO%20449994dfdd3a40b9b04b9003d4b0fc7f/%25D0%25A1%25D0%25BD%25D0%25B8%25D0%25BC%25D0%25BE%25D0%25BA_%25D1%258D%25D0%25BA%25D1%2580%25D0%25B0%25D0%25BD%25D0%25B0_2022-10-20_%25D0%25B2_22.05.37.png)

### Почему же так происходит?

Все дело в том, как устроена map в go. Map в go это неупорядоченная структура данных(не хочу называть это коллекцией), содержащая пары ключей и значений, где все ключи различны. Map реализует hash table, с помощью  массива, где каждый элемент это ссылка(pointer) на bucket(фактически ключ и значение).

![Untitled](Maps%20%D0%B8%20%D1%83%D1%82%D0%B5%D1%87%D0%BA%D0%B8%20%D0%BF%D0%B0%D0%BC%D1%8F%D1%82%D0%B8%20%D0%B2%20GO%20449994dfdd3a40b9b04b9003d4b0fc7f/Untitled.png)

Bucket фиксированного размера, в 8 элементов. В случаи вставки в заполненный bucket, go создает новый bucket и соединяет их с предыдущим.

![Untitled](Maps%20%D0%B8%20%D1%83%D1%82%D0%B5%D1%87%D0%BA%D0%B8%20%D0%BF%D0%B0%D0%BC%D1%8F%D1%82%D0%B8%20%D0%B2%20GO%20449994dfdd3a40b9b04b9003d4b0fc7f/Untitled%201.png)

Реализовано это в пакете runtime. Наша map это структура hmap. Структура эта содержит несколько полей, но причиной данной утечки памяти является поле B.

```go
// A header for a Go map.
type hmap struct {
	// Note: the format of the hmap is also encoded in cmd/compile/internal/reflectdata/reflect.go.
	// Make sure this stays in sync with the compiler's definition.
	count     int // # live cells == size of map.  Must be first (used by len() builtin)
	flags     uint8
	B         uint8  // log_2 of # of buckets (can hold up to loadFactor * 2^B items)
	noverflow uint16 // approximate number of overflow buckets; see incrnoverflow for details
	hash0     uint32 // hash seed

	buckets    unsafe.Pointer // array of 2^B Buckets. may be nil if count==0.
	oldbuckets unsafe.Pointer // previous bucket array of half the size, non-nil only when growing
	nevacuate  uintptr        // progress counter for evacuation (buckets less than this have been evacuated)

	extra *mapextra // optional fields
}
```

> Внимательный читатель уже скорее всего догадался в чем причина, и объяснять это не требуется. Но важно сказать, что map это ссылочный тип, и лучше не использовать map как глобальную переменную.
>

Поле B это количество buckets. После добавления элементов оно равно логорифму от количества buckets. После удаления элементов, это поле остается прежним. Мы удалили данные, но память осталась зарезервированной под нужное количество элементов. Поэтому map может расти и не освобождать память до конца.

Вызов сборщика мусора не повлияет на данную ситуацию, при такой конфигурации map. Это может стать проблемой если вы решили сделать кэш в своем приложении для большого количества элементов.

## Как избежать проблем?

Всего может быть два сценария, не считая перезапуска приложения.

1. После удаления не нужных элементов создать новую map и скопировать в нее данные
2. Изменить тип данных map. Например так: **m := make(map[int]*[128]byte)**

В первом случаи, это сработает потому что мы просто не создадим не нужное количество buckets. Во втором, нам поможет GC, так как теперь наши данные находятся на\в куче, и GC сможет подчистить за нами мусор.

---

<aside>
<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAOEAAADhCAMAAAAJbSJIAAAAeFBMVEX/AAD/////u7v/6+v/0ND/aWn/mZn/zc3/9fX/h4f/5ub/+Pj/tbX/t7f/oKD/ODj/Kir/3d3/wMD/MzP/UVH/1dX/yMj/qan/lJT/TU3/gID/Fxf/PT3/eHj/Dg7/jY3/ZWX/Q0P/XV3/IiL/cXH/q6v/X1//eXnZjuaVAAAE20lEQVR4nO2d63aiQAyAOyKgXAVF0Lbrtbvv/4YLZ9didUDQQEjI9689RyffwcCQub0p7rxhB9A5YkgfMaSPGNJHDOkjhvQRQ/qIIX3EkD5iSB8xpI8Y0kcM6SOG9BFD+oghfcSQPmJIHzGkjxi+jHEhml4Tff+/6wBeMXTdWU6SrD0/DQIry3Z709yEYbh4e45F/tmNae53WWYFQep76yQpmnDd3gxdw4mi2LPm27O5OZzenzRpy/vpsDHP27nlxVHkGO18Gxo6dmx9HjfLvpzqeF9ujp9WbDtQhnZqPvur65qFmdovGjrBCtviIaug/mLWGdp77Ogbsq+7ktWG0fAvX8kqam+4xQ66JduWhk6IHXFrwop01BuuscN9inVzQw871ifxmhr62JE+jd/MMMGO8wWSJoYudpQvcd9nvTc8Ygf5EsfHhlTvMhfu7ja3hu4JO8QXOd3+Tm8NA+wIXyZ4YIgdHwD1htSzsMCrNTxghwfAoc7QwI4OBKPGcIIdHAiTGsMzdnAgnGsMP7CDA+Gj2tDBjg0Ip9LQxg4NCLvS0MIODQir0vATOzQgPisNedxobm4114Yz7MjAmFUY8ujRFBgVhjRriDrWFYYpdmBgpBWGXB4WPx8X14Y8eqUF5wrDJXZgYCwrDLHjAmSshnwehz8eiFeGlMcrbkm0hjF0M4hTOGKtIfig2iRGm37jaw3Ba6UTpTLo72yIpzWcQzdT1LwMnG7EXGtoQjfzr6qXbKC/twGm1hB84PBSt4x/QX/zQ45aQ/A3/LIy23uf/kNrCH5zv6o9Gz2XgBa9GyoV9ZqOWsMZeLb8HD9QcY/Dy79mGIZ9pqPWMAJv5s5QGb3NB4yQDPub06kznIK3ojPM07GXDvkU0VCpoIcegM4QfuCpylDNwLvAd5TDTyiGSjldp6POEL7iXWOYN9dtZa+sepeGX+Ct1BoqlXaZjl8aQ/ia/gPDTtOxrOtjGubp2NlUT53hDryVx4Z5OnY0KrvTGML/ZJoY5vnfSTqWZQx8w27ScVCGeTr+Bm9bZwjf7W9smKcj9KTIcpFQaQheamtjqJQHm45lsW0whsBpMkhDZezh2h6mYf4GB5aOQzVUygcazBmuoVIwd5zhGnK/htzzkP29lPvzsIc+Da9+6R+NIf93C3k/bM/Q3vH512n419r410v517z5j1vwH3viP37IfwyY/zg+/7kY/OfT8J8TxX9eG/+5iSOYX8p/jjD/ed785+rzX2/Bf80M/3VP/Neu8V9/yH8NKf91wCMw5L8en/+eCvz3xeC/twn//Wn4PBCr9hjiv08U/72+RrBfG5fHRfWee/z3TeS/9yWTW03N/qVMeqZ1e9Dy30eYR6+mbi9o/vt5s9iT3a815FDJuDW6+Zv/2Qj8z7cgn4kPzygZwTkzIzgriPT4RaPznij33XSzsHSGZO82jc9do1pXbHF23gjOP1T8z7BUIziHVPE/S7aA+3nA/68k6zOdy4vJ+Vzuaxifra73dYvD65Nk7flpEFhZttub5ioMw2d/1Yv8syvT3O+yzAqC1PfsJCmacNs5wRk2wrgQTa+Jvv/fdQCdG6IjhvQRQ/qIIX3EkD5iSB8xpI8Y0kcM6SOG9BFD+oghfcSQPmJIHzGkjxjSRwzpI4b0EUP6iCF9/gKCNIdvAhrz/wAAAABJRU5ErkJggg==" alt="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAOEAAADhCAMAAAAJbSJIAAAAeFBMVEX/AAD/////u7v/6+v/0ND/aWn/mZn/zc3/9fX/h4f/5ub/+Pj/tbX/t7f/oKD/ODj/Kir/3d3/wMD/MzP/UVH/1dX/yMj/qan/lJT/TU3/gID/Fxf/PT3/eHj/Dg7/jY3/ZWX/Q0P/XV3/IiL/cXH/q6v/X1//eXnZjuaVAAAE20lEQVR4nO2d63aiQAyAOyKgXAVF0Lbrtbvv/4YLZ9didUDQQEjI9689RyffwcCQub0p7rxhB9A5YkgfMaSPGNJHDOkjhvQRQ/qIIX3EkD5iSB8xpI8Y0kcM6SOG9BFD+oghfcSQPmJIHzGkjxi+jHEhml4Tff+/6wBeMXTdWU6SrD0/DQIry3Z709yEYbh4e45F/tmNae53WWYFQep76yQpmnDd3gxdw4mi2LPm27O5OZzenzRpy/vpsDHP27nlxVHkGO18Gxo6dmx9HjfLvpzqeF9ujp9WbDtQhnZqPvur65qFmdovGjrBCtviIaug/mLWGdp77Ogbsq+7ktWG0fAvX8kqam+4xQ66JduWhk6IHXFrwop01BuuscN9inVzQw871ifxmhr62JE+jd/MMMGO8wWSJoYudpQvcd9nvTc8Ygf5EsfHhlTvMhfu7ja3hu4JO8QXOd3+Tm8NA+wIXyZ4YIgdHwD1htSzsMCrNTxghwfAoc7QwI4OBKPGcIIdHAiTGsMzdnAgnGsMP7CDA+Gj2tDBjg0Ip9LQxg4NCLvS0MIODQir0vATOzQgPisNedxobm4114Yz7MjAmFUY8ujRFBgVhjRriDrWFYYpdmBgpBWGXB4WPx8X14Y8eqUF5wrDJXZgYCwrDLHjAmSshnwehz8eiFeGlMcrbkm0hjF0M4hTOGKtIfig2iRGm37jaw3Ba6UTpTLo72yIpzWcQzdT1LwMnG7EXGtoQjfzr6qXbKC/twGm1hB84PBSt4x/QX/zQ45aQ/A3/LIy23uf/kNrCH5zv6o9Gz2XgBa9GyoV9ZqOWsMZeLb8HD9QcY/Dy79mGIZ9pqPWMAJv5s5QGb3NB4yQDPub06kznIK3ojPM07GXDvkU0VCpoIcegM4QfuCpylDNwLvAd5TDTyiGSjldp6POEL7iXWOYN9dtZa+sepeGX+Ct1BoqlXaZjl8aQ/ia/gPDTtOxrOtjGubp2NlUT53hDryVx4Z5OnY0KrvTGML/ZJoY5vnfSTqWZQx8w27ScVCGeTr+Bm9bZwjf7W9smKcj9KTIcpFQaQheamtjqJQHm45lsW0whsBpMkhDZezh2h6mYf4GB5aOQzVUygcazBmuoVIwd5zhGnK/htzzkP29lPvzsIc+Da9+6R+NIf93C3k/bM/Q3vH512n419r410v517z5j1vwH3viP37IfwyY/zg+/7kY/OfT8J8TxX9eG/+5iSOYX8p/jjD/ed785+rzX2/Bf80M/3VP/Neu8V9/yH8NKf91wCMw5L8en/+eCvz3xeC/twn//Wn4PBCr9hjiv08U/72+RrBfG5fHRfWee/z3TeS/9yWTW03N/qVMeqZ1e9Dy30eYR6+mbi9o/vt5s9iT3a815FDJuDW6+Zv/2Qj8z7cgn4kPzygZwTkzIzgriPT4RaPznij33XSzsHSGZO82jc9do1pXbHF23gjOP1T8z7BUIziHVPE/S7aA+3nA/68k6zOdy4vJ+Vzuaxifra73dYvD65Nk7flpEFhZttub5ioMw2d/1Yv8syvT3O+yzAqC1PfsJCmacNs5wRk2wrgQTa+Jvv/fdQCdG6IjhvQRQ/qIIX3EkD5iSB8xpI8Y0kcM6SOG9BFD+oghfcSQPmJIHzGkjxjSRwzpI4b0EUP6iCF9/gKCNIdvAhrz/wAAAABJRU5ErkJggg==" width="40px" /> Большое спасибо за прочтение. Оригинальная статья находится в самом начале текста. 

Так же вы можете подписаться на мой youtube канал.

</aside>

[Gleb Nazemnov](https://www.youtube.com/channel/UC0y2imcpHCM97-MV1lUQvKg/featured)