package main

import (
	"encoding/csv"
	"golang.org/x/exp/slices"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"gopkg.in/ini.v1"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

/*====================================================================================================================*/

// Member Структура члена собрания для вывода в таблицу
type Member struct {
	//Группа - первая сортировка
	Group string
	//ФИО - вторая сортировка
	FullName string
	//Пометка об опоздании
	Delay string
	//Пометка о раннем или позднем выходе с собрания
	EarlyExit string
	//Пометка о присутствии (или отсутствии)
	Presence string
}

// Header Структура оглавления отчёта
type Header struct {
	//Название собрания
	Title string
	//Дата проведения собрания
	Date string
	//Номер пары
	LessonNumber string
}

/*====================================================================================================================*/

// SetConfigurations Функция, считывающая конфигурации путей до загрузок и до директории будущего расположения отчёта
func SetConfigurations() (string, string) {
	//Определяем ОС пользователя
	currentOS := runtime.GOOS
	//Открываем .ini файл
	configurationFile, err := ini.Load("cfg.ini")
	if err != nil {
		log.Fatalf("Ошибка открытия файла конфигураций: %v", err)
	}

	//Считываем из файла конфигураций пути до загрузок и будущего расположения отчёта
	downloadFolderPath := configurationFile.Section("paths").Key("download_folder_path").String()
	reportLocationPath := configurationFile.Section("paths").Key("report_location_folder").String()

	//Если значение для пути до загрузок не установлено, ставим значение по-умолчанию в зависимости от ОС пользователя
	if downloadFolderPath == "" {
		switch {
		//Для Windows путь до папки загрузок по-умолчанию "C:\\Users\\user\\Downloads\\"
		case currentOS == "windows":
			downloadFolderPath = "C:\\Users\\user\\Downloads\\"
		//Для Linux путём по-умолчанию является текущая директория "."
		case currentOS == "linux":
			downloadFolderPath = "."
		//Для MacOS путём до загрузок по умолчанию является "~/Downloads/"
		case currentOS == "darwin":
			downloadFolderPath = "~/Downloads/"
		}
	}

	//Если значения для пути до будущего расположения отчёта не установлено, ставим значение по-умолчанию в зависимости
	//от ОС пользователя
	if reportLocationPath == "" {
		switch {
		//Для Windows путём по-умолчанию является рабочий стол
		case currentOS == "windows":
			reportLocationPath = "C:\\Users\\user\\Desktop\\"
		//Для Linux путём по умолчанию является текущая директория
		case currentOS == "linux":
			reportLocationPath = "."
		//Для MacOS путём по умолчанию является рабочий стол
		case currentOS == "darwin":
			reportLocationPath = "~/Desktop/"
		}
	}

	//В зависимости от ОС возвращаем пути до каталогов загрузок и размещения с припиской корректных слэшей с целью
	//предотвращения ошибок поиска пути
	if currentOS == "windows" {
		return downloadFolderPath + "\\", reportLocationPath + "\\"
	} else {
		return downloadFolderPath + "/", reportLocationPath + "/"
	}
}

/*====================================================================================================================*/

// FormCSVList Вспомогательная функция, которая возвращает список .csv файлов из загрузок
func FormCSVList(root string) []string {
	//Массив всех найденных .csv файлов
	var csvFiles []string

	//Считываем директорию в массив dir, элементы dir являются fs.FileStat
	dir, err := ioutil.ReadDir(root)
	//Стандартная проверка на ошибку при чтении директории (файла)
	if err != nil {
		log.Fatalf("Ошибка открытия директории: %v", err)
	}

	//Цикл по всем элементам массива dir
	for _, file := range dir {
		//Условие: если элемент file НЕ является директорией и его расширение .csv
		if !file.IsDir() && filepath.Ext(file.Name()) == ".csv" {
			//В конец массива добавляется строка, содержащая полный путь до .csv файла
			csvFiles = append(csvFiles, root+file.Name())
		}
	}

	//Если по-указанному в cfg.ini пути до загрузок не оказалось .csv файлов, то выводится ошибка и команда завершает свою работу
	if len(csvFiles) == 0 {
		log.Fatalf("В данном каталоге не содержится .csv файлов, вероятно, неверно указан путь до загрузок")
	}

	return csvFiles
}

// FindCurrentReport Функция, которая возвращает текущий (последний) .csv файл
func FindCurrentReport(root string) string {
	//Формируем список .csv файлов с помощью функции FormCSVList()
	csvFiles := FormCSVList(root)

	//Присваиваем первый элемент списка .csv файлов необходимому отчёту для дальнейшего поиска текущего отчёта
	//(Присваиваем первый элемент, т.к. первым элементом массив чаще всего является последний файл)
	report := csvFiles[0]

	//Цикл по всем элементам массива .csv файлов, за исключением 1 элемента
	for i := 1; i < len(csvFiles); i++ {
		//Считываем i-тый элемент массива в виде os.Stat, для получения подробной информации о файле
		temp, err := os.Stat(csvFiles[i])
		if err != nil {
			log.Fatalf("Ошибка открытия файла: %v", err)
		}

		//Считываем текущий отчёт в виде os.Stat
		currentReport, err := os.Stat(report)
		if err != nil {
			log.Fatalf("Ошибка открытия файла: %v", err)
		}

		//Условие: если последняя модификация i-того элемента массива была позже текущего отчёта
		if temp.ModTime().After(currentReport.ModTime()) {
			//Текущий отчёт становится i-тым элементом списка
			report = root + temp.Name()
		}
	}

	return report
}

/*====================================================================================================================*/

// ParseTime Вспомогательная функция, возвращающая время в секундах в виде целочисленного значения
func ParseTime(words []string) int {
	//Если массив строк содержит 3 переменные (часы, минуты, секунды)
	if int(len(words)) == 3 {
		//Переводим первый элемент строкового массива (часы) в целочисленное значение
		hours, err := strconv.Atoi(words[0])
		if err != nil {
			log.Fatalf("Ошибка перевода строки часов в десятичное число: %v", err)
		}

		//Переводим второй элемент строкового массива (минуты) в целочисленное значение
		minutes, err := strconv.Atoi(words[1])
		if err != nil {
			log.Fatalf("Ошибка перевода строки минут в десятичное число: %v", err)
		}

		//Переводим третий элемент строкового массива (секунды) в целочисленное значение
		time, err := strconv.Atoi(words[2])
		if err != nil {
			log.Fatalf("Ошибка перевода строки секунд в десятичное число: %v", err)
		}

		//Возвращаем количество секунд
		return time + hours*3600 + minutes*60
		//Иначе массив содержит две строковые переменные (или меньше, но такие ситуации не рассматриваются)
	} else {
		//Переводим первый элемент строкового массива (минуты) в целочисленное значение
		minutes, err := strconv.Atoi(words[0])
		if err != nil {
			log.Fatalf("Ошибка перевода строки минут в десятичное число: %v", err)
		}

		//Переводим второй элемент строкового массива (секунды) в целочисленное значение
		time, err := strconv.Atoi(words[1])
		if err != nil {
			log.Fatalf("Ошибка перевода строки секунд в десятичное число: %v", err)
		}

		//Возвращаем количество секунд
		return time + minutes*60
	}
}

// ParseLessonNumberOrDelay Функция, которая переводит строку времени в номер пары
//Так же функция обрабатывает опоздание
func ParseLessonNumberOrDelay(source, phase string) string {
	//Массив из трёх переменных, полученных из строки времени путём деления по двоеточию
	words := strings.Split(source, ":")

	//Получаем время в секундах с помощью вспомогательной функции ParseTime()
	time := ParseTime(words)

	//Если фаза = заполнение оглавления
	if phase == "header" {
		//Разбор ситуаций. Если время начала собрания в секундах лежит в пределах [начало пары -15 минут и конец пары +15 минут],
		//то из функции возвращается номер пары, в случае, если ни одного случая не подходят, возвращается Консультация
		switch {
		//Диапазон пары +- 15 минут
		case time >= 27800 && time <= 35100:
			return "Пара 1"
		case time >= 33900 && time <= 41100:
			return "Пара 2"
		case time >= 39900 && time <= 47100:
			return "Пара 3"
		case time >= 46700 && time <= 53300:
			return "Пара 4"
		case time >= 53100 && time <= 60300:
			return "Пара 5"
		case time >= 59100 && time <= 66300:
			return "Пара 6"
		case time >= 65100 && time <= 72300:
			return "Пара 7"
		case time >= 70700 && time <= 77900:
			return "Пара 8"
		default:
			return "Консультация"
		}
		//Если фаза = заполнению члена собрания
	} else {
		//Разбор ситуации. Если время присоединения позже 5 минут от начала пары, то опоздание, иначе без опоздания
		switch {
		case time >= 29000 && time <= 35100:
			return "Опоздал"
		case time >= 35100 && time <= 41100:
			return "Опоздал"
		case time >= 41100 && time <= 47100:
			return "Опоздал"
		case time >= 47900 && time <= 53300:
			return "Опоздал"
		case time >= 54300 && time <= 60300:
			return "Опоздал"
		case time >= 60300 && time <= 66300:
			return "Опоздал"
		case time >= 66300 && time <= 72300:
			return "Опоздал"
		case time >= 71900 && time <= 77900:
			return "Опоздал"
		default:
			return "Без опоздания"
		}
	}
}

// GetDateAndLessonNumberOrDelay Функция, обрабатывающая строку с датой и временем начала собрания, и возвращающая
// их по-отдельности. Так же в функцию поступает значение фазы, которое позволяет применить функцию для
// определения опоздания
func GetDateAndLessonNumberOrDelay(source, phase string) (string, string) {
	//Разделяем строку с датой и временем по запятой
	words := strings.Split(source, ",")

	//Убираем лишний пробел в начале строки времени
	words[1] = strings.ReplaceAll(words[1], " ", "")

	//fmt.Println(words)
	//Если параметр фазы = заполнению оглавления
	if phase == "header" {
		//Переменная, содержащая дату
		date := words[0]

		//Номер пары получается из строки времени и сопоставляется со временем начала и конца пары (+-15 минут)
		lessonNumber := ParseLessonNumberOrDelay(words[1], phase)

		return date, lessonNumber
		//Если параметр фазы = заполнение члена собрания
	} else {
		//Пометка об опоздании возвращается из функции ParseLessonNumberOrDelay (второе значение - пустое)
		return ParseLessonNumberOrDelay(words[1], phase), "_"
	}
}

// GetDurationOfPresence Функция, обрабатывающая строку нахождения участника на собрании и возвращающая пометку
//о малом или полном нахождении на собрании
func GetDurationOfPresence(source string) string {
	//Разбиваем строку на массив строк по символам пробела
	words := strings.Fields(source)

	//Если массив состоит из двух строк, то участник находился на собрании меньше минуты, следовательно,
	// на паре почти не присутствовал
	if len(words) == 2 {
		return "Малое присутствие на паре"
		//Если массив состоит из 4 строк, то участник был на собрании менее часа, но больше минуты. Требуется обработка
	} else if len(words) == 4 {
		//Вспомогательный массив, содержащий только строки чисел
		timeArray := []string{words[0], words[2]}

		//Получаем время в секундах с помощью функции ParseTime()
		time := ParseTime(timeArray)

		//Разбор ситуации. Если время больше 30 минут, то участник считается полноценным участником собрания,
		// иначе ставится пометка о малом нахождении на собрании
		switch {
		//Время присутствия на паре более 30 минут
		case time > 1800:
			return "Полное присутствие на паре"
		default:
			return "Малое нахождение на паре"
		}
		//Иначе массив состоит из 6 или более строк, т.е. больше часа, следовательно участник находился на паре
		// полное время
	} else {
		return "Полное присутствие на паре"
	}
}

// SetGroup Функция, устанавливающая группу участника собрания, на основе базы групп и ФИО участника
func SetGroup(fullName string) string {
	//Открываем файл с базой групп
	file, err := os.Open("GroupsBase.csv")
	if err != nil {
		log.Fatalf("Ошибка открытия файла базы групп: %v", err)
	}

	//Закрываем файл после окончания функции
	defer file.Close()

	//Читаем поток данных из базы групп
	reader := csv.NewReader(file)

	//Цикл по всем строкам в файле
	for {
		//Считываем строку из базы групп
		currentDataRow, err := reader.Read()
		//При окончании файла выходим из цикла
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Ошибка чтения из файла базы групп: %v", err)
		}

		//Условие, если текущий член базы групп совпадает по ФИО с поступившим на исполнение функции участником собрания
		if currentDataRow[0] == fullName {
			//Если условие выполнено, то группой участника собрания становится группа текущего члена базы групп
			return currentDataRow[1]
		}
	}

	//В случае, если в базе нет данного пользователя, то участник собрания маркируется гостем
	return "Гость"
}

// ReadCSVReport Функция, которая парсит отчёт на две структуры: оглавление отчёта и массив членов собрания
func ReadCSVReport(report string) (Header, []Member) {
	//Считываем отчёт
	file, err := os.Open(report)
	if err != nil {
		log.Fatalf("Ошибка открытия файла1: %v", err)
	}

	//Закрываем файл
	defer file.Close()

	//Генерируем декодер для UTF-16 Little-Endian с BOM
	dec := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()

	//Создаём новый поток данных и файла с отчётом, но с кодировкой UTF-8 с BOM
	utf8r := transform.NewReader(file, dec)

	//Переменная, читающая .csv файл
	data := csv.NewReader(utf8r)

	//Отчёты от MS Teams разделяются символом табуляции, устанавливаем деление на символ табуляции
	data.Comma = '\t'

	//Убираем количество полей в Reader, чтобы не возникало ошибок о некорректном количество полей в строке
	data.FieldsPerRecord = -1

	//Переменная оглавления
	var header Header

	//Цикл по первым 8 строкам .csv файла, которые меняются только в названии собрания, дате и времени начала
	// и конца собрания. Цикл формирует структуру со всеми данными оглавления отчёта
	for i := 0; i < 8; i++ {
		//Считываем строку отчёта
		row, err := data.Read()
		if err != nil {
			log.Fatalf("Ошибка чтения строки csv файла: %v", err)
		}

		//Разбор ситуации. В зависимости от номера строки заполняется структура оглавления (или строка пропускается)
		switch {
		//В третьей строке указано название собрания
		case i == 2:
			//Заполняем поле название собрания второй колонки из отчёта
			//Если название собрания не было изменено вручную или не было введено, ему присваивается
			// "Название по-умолчанию"
			if len(row) > 1 {
				if row[1] == "General" {
					header.Title = "Название по-умолчанию"
				} else {
					header.Title = row[1]
				}
			} else {
				header.Title = "Название по-умолчанию"
			}
		//В четвёртой строке указаны дата и время начала собрания
		case i == 3:
			//Заполняются поля с датой проведения пары и номером пары с помощью вспомогательного метода
			// GetDateAndLessonNumber()
			header.Date, header.LessonNumber = GetDateAndLessonNumberOrDelay(row[1], "header")
		//Во всех остальных строках оглавления не содержится необходимой информации, они пропускаются
		default:
		}
	}

	//Массив, содержащий всех членов собрания
	var members []Member

	//Безусловный цикл, в котором будет заполняться массив членов собрания
	for {
		//Считываем строку из .csv файла
		row, err := data.Read()

		//Если обнаружен конец файла, то цикл прерывается
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Ошибка чтения строки csv файла: %v", err)
		}

		//Переменная, в которую будет записываться данные из текущей строки отчёта
		var currentMember Member

		//Если член собрания является инициатором(преподавателем), то он пропускается
		if row[5] != "Инициатор" {
			//Разбиваем 1 элемент строки на отдельные строки ФИО
			fullNameArr := strings.Fields(row[0])

			//Если длина массива ФИО больше 1, приводим ИОФ к ФИО. Проверка на длину исключает ряд ошибок, связанных с
			//некорректной регистраций на собрание
			if len(fullNameArr) > 1 {
				//Меняем местами строки, чтобы перейти к виду ФИО
				fullNameArr[0], fullNameArr[1], fullNameArr[2] = fullNameArr[2], fullNameArr[0], fullNameArr[1]
			} else {
				//В случае, если имя участника собрания написано слитно - это ошибка регистрации на собрание, из данного
				// пользователя нельзя получить корректной информации. Возвращение в начала цикла
				continue
			}

			//Цикл по всем индексам массива имени участника собрания для выборки групп, при некорректном регестрировании
			for i := range fullNameArr {
				//Убираем из имени пометку (гость), установленную Teams
				if fullNameArr[i] == "(гость)" || fullNameArr[i] == "(Guest)" {
					fullNameArr[i] = ""
				}
				//Перменная являющаяся группой в некорректном имени
				mayBeGroup := strings.ReplaceAll(strings.ToLower(strings.Split(fullNameArr[i], "-")[0]), "(", "")
				//Если буквенная аббривиатура присутствует в имени, условие выполняется
				if mayBeGroup == "мп" || mayBeGroup == "мт" || mayBeGroup == "мк" || mayBeGroup == "мн" {
					//Избавляемся от лишник скобок (при наличии)
					fullNameArr[i] = strings.ReplaceAll(fullNameArr[i], ")", "")
					//Устанавливаем группу текущему участнику с некорректным именем
					currentMember.Group = fullNameArr[i]
				}
			}

			//Соединяем массив в единую строку
			fullName := strings.Join(fullNameArr, " ")

			//Устанавливаем ФИО участника
			currentMember.FullName = fullName

			//Если группа у текущего участника собрания не установлена, устанавливаем
			if currentMember.Group == "" {
				//Устанавливаем группу у конкретного участника собрания с помощью вспомогательной функции SetGroup()
				currentMember.Group = SetGroup(currentMember.FullName)
			}

			//Пометка об опоздании поступает из функции GetDateAndLessonNumberOrDelay (второе значение пустое)
			//На вход в функцию подаётся время присоединения участника к собранию
			currentMember.Delay, _ = GetDateAndLessonNumberOrDelay(row[1], "member")

			//Пометка о малом нахождении на паре (Если меньше получаса - малое присутствие на паре, иначе полное)
			currentMember.EarlyExit = GetDurationOfPresence(row[3])

			//Если стоит пометка о малом нахождении на паре, то ставится пометка об отсутствии на паре
			if currentMember.EarlyExit == "Полное присутствие на паре" {
				currentMember.Presence = "Присутствовал"
			} else {
				currentMember.Presence = "Присутствовал не полностью"
			}

			//Добавляем сформированного студента в список всех студентов
			members = append(members, currentMember)
		}
	}

	return header, members
}

/*====================================================================================================================*/

// FormReport Функция, формирующая отчёт в виде .csv файла. Принимает на вход созданное оглавление отчёта и список всех
//участников собрания, за исключением инициатора(преподавателя)
func FormReport(header Header, members []Member, reportLocationPath string) {
	//Переменная, содержащая полный путь до сформированного отчёта. Название формируется из названия и даты проведения
	formedReportRoot := reportLocationPath + "Отчёт о проведение собрания_" + header.Title + "_" + header.Date + ".csv"

	//Создаём файл по сформированному пути
	file, err := os.Create(formedReportRoot)
	if err != nil {
		log.Fatalf("Ошибка создания файла: %v", err)
	}

	//Закрываем файл по окончанию функции
	defer file.Close()

	//Данная строка указывает на то, что файл записан в кодировки UTF-8 c BOM, т.к. только в такой кодировки MS Exel
	//корректно отображает кириллицу
	_, err = file.WriteString("\xEF\xBB\xBF")
	if err != nil {
		log.Fatalf("Ошибка записи строки с кодировкой: %v", err)
	}

	//Создаём писец .csv файлов
	csvWriter := csv.NewWriter(file)

	//Устанавливаем разделитель писца на точку с запятой
	csvWriter.Comma = ';'

	//Отчищаем буфер писца по окончанию функции
	defer csvWriter.Flush()

	//Цикл по количеству строк оглавления отчёта
	for i := 0; i < 3; i++ {
		//Разбор ситуации.
		switch {
		//Первая строка содержит название собрания(пары)
		case i == 0:
			//Создаём массив со строкой, который будет записываться в отчёт. Базовая строка:"Название собрания";
			//Название собрания из отчёта (Массив необходим для записи в файл)
			headerComponent := []string{"Название собрания", header.Title}
			//Записываем массив в строку в отчёт
			if err := csvWriter.Write(headerComponent); err != nil {
				log.Fatalf("Ошибка записи строки названия собрания: %v", err)
			}
		//Вторая строка содержит дату проведения собрания(пары)
		case i == 1:
			//Создаём массив со строкой, который будет записываться в отчёт. Базовая строка:"Дата проведения собрания";
			//Дата собрания из отчёта
			headerComponent := []string{"Дата проведения собрания", header.Date}
			if err := csvWriter.Write(headerComponent); err != nil {
				log.Fatalf("Ошибка записи даты проведения собрания: %v", err)
			}
		//Третья строка содержит номер пары
		case i == 2:
			//Создаём массив со строкой, который будет записываться в отчёт. Базовая строка:"Номер пары";
			//Номер пары получается из времени проведения собрания
			headerComponent := []string{"Номер пары", header.LessonNumber}
			if err := csvWriter.Write(headerComponent); err != nil {
				log.Fatalf("Ошибка записи строки номера пары: %v", err)
			}
		}
	}

	//Записываем в отчёт пустую строку, чтобы отделить оглавление от списка участников собрания
	if err := csvWriter.Write([]string{""}); err != nil {
		log.Fatalf("Ошибка записи пустой строки: %v", err)
	}

	//"Шапка" таблицы участников собрания(студентов)
	memberHeader := []string{"Группа", "ФИО", "Присутствие", "Опоздание", "Время нахождения на собрании"}

	//Записываем "шапку" таблицы участников собрания(студентов)
	if err := csvWriter.Write(memberHeader); err != nil {
		log.Fatalf("Ошибка записи строки шапки участников: %v", err)
	}

	//Цикл по всем участникам собрания
	for i := 0; i < len(members); i++ {
		//Если i-тый участник собрания - пустой, т.е. инициатор(преподаватель), он пропускается в записи
		if members[i].FullName != "" {
			//Создаём массив со строкой, которая будет записываться в отчёт. Массив состоит из всех данных участника собрания(студента)
			memberInformation := []string{members[i].Group, members[i].FullName, members[i].Presence, members[i].Delay, members[i].EarlyExit}
			//Записываем массив в строку в отчёт
			if err := csvWriter.Write(memberInformation); err != nil {
				log.Fatalf("Ошибка записи строки участника собрания: %v", err)
			}
		}
	}
}

/*====================================================================================================================*/

// FillLostMembers Функция, заполняющая массив участников собрания людьми, которые не присутствовали на собрании
func FillLostMembers(members []Member) []Member {
	//Массив, в который будут записаны все уникальные группы
	var groups []string

	//Цикл по всем переменным массива members для нахождения уникальных групп
	for _, currentGroup := range members {
		//Переменная, отслеживающая повторение группы
		skip := false

		//Цикл по всем уникальным группам
		for _, uniqGroup := range groups {
			//Если группа текущего участника собрания уже встречалась, переменная, отвечающая за уникальность меняет значение
			//и цикл прерывается
			if currentGroup.Group == uniqGroup {
				skip = true
				break
			}
		}

		//Если группа уникальна, она добавляется в массив уникальных групп
		if !skip {
			groups = append(groups, currentGroup.Group)
		}
	}

	//Открываем файл с базой групп
	file, err := os.Open("GroupsBase.csv")
	if err != nil {
		log.Fatalf("Ошибка открытия файла базы групп: %v", err)
	}

	//Закрываем файл после окончания функции
	defer file.Close()

	//Читаем данный из файла базы групп
	reader := csv.NewReader(file)

	//Карта (ключ - значение) для составления списка всех участников
	baseMembers := make(map[string]bool)

	//Цикл по всем строкам файла базы групп
	for {
		//Считываем строку из базы групп
		row, err := reader.Read()
		//Если файл закончился - выходим из цикла
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Ошибка открытия файла базы групп: %v", err)
		}

		//Если группа текущего студента из базы совпадает с одной из уникальных групп, то условие выполняется
		if slices.IndexFunc(groups, func(group string) bool { return group == row[1] }) != -1 {
			//Заполняем карту с ключом - ФИО, значение НЕ истины
			baseMembers[row[0]] = false
		}
	}

	//Цикл по всем студентам, студенты из чьих группы были на собрании
	for curMember := range baseMembers {
		//Условие, если студент из группы был на собрании, то он помечается как присутствующий
		if slices.IndexFunc(members, func(members Member) bool { return curMember == members.FullName }) != -1 {
			baseMembers[curMember] = true
		}
	}

	//Цикл по всем студентам, студенты из чьих группы были на собрании
	for curMember := range baseMembers {
		//Условие, если у студента стоит пометка о том, что его не было, то условие проходит
		if baseMembers[curMember] == false {
			//Создаётся новый участник собрания
			var newMember Member

			//ФИО отсутствующего студента является ФИО из базы
			newMember.FullName = curMember

			//Группа устанавливается с помощью функции SetGroup()
			newMember.Group = SetGroup(newMember.FullName)

			//Ставится пометка о полном отсутствии
			newMember.Presence = "Отсутствовал"

			//Отсутствующий студент заносится в список
			members = append(members, newMember)
		}
	}

	return members
}

/*====================================================================================================================*/

// SortMembers Функция, совершающая двойную сортировку списка участников собрания сначала по группам, потом по ФИО
func SortMembers(members []Member) {
	//Сортировка массива структур с помощью встроенной в GO функции сортировки
	sort.Slice(members, func(i, j int) (less bool) {
		return members[i].FullName < members[j].FullName
	})

	//Сортировка массива структур с помощью встроенной в GO функции сортировки, сохраняя оригинальный порядок
	// незатронутых полей или равные элементы
	sort.SliceStable(members, func(i, j int) (less bool) {
		return members[i].Group < members[j].Group
	})
}

/*====================================================================================================================*/

func main() {
	//Считываем конфигурации путей до загрузок и пути сохранения отчёта
	downloadPath, reportLocationPath := SetConfigurations()

	//Находим текущий отчёт с помощью функции FindCurrentReport()
	report := FindCurrentReport(downloadPath)

	//Формируем оглавление и список участников собрания с помощью функции ReadCSVReport()
	header, members := ReadCSVReport(report)

	//Заполняем массив участников собрания людьми, которых не было на собрании с помощью функции FillLostMembers(),
	// если собрание не было консультацией
	if header.LessonNumber != "Консультация" {
		members = FillLostMembers(members)
	}

	//Сортируем список участников собрания с помощью функции SortMembers()
	SortMembers(members)

	//Формируем и заполняем отчёт в виде .csv файла с помощью функции FormReport()
	FormReport(header, members, reportLocationPath)
}
