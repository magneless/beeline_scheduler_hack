from pathlib import Path
import json, math, re, os
import fitz
from PIL import Image, ImageOps, ImageDraw

ROOT = Path(__file__).resolve().parent.parent
(ROOT / 'preview').mkdir(parents=True, exist_ok=True)
W,H = 960,540
def font_path(variable, filename):
    configured = os.environ.get(variable)
    candidates = [Path(configured)] if configured else [
        Path('/mnt/c/Windows/Fonts') / filename,
        Path('C:/Windows/Fonts') / filename,
    ]
    for candidate in candidates:
        if candidate.is_file():
            return str(candidate)
    raise FileNotFoundError(f'Arial font not found; set {variable} to {filename}')

FONT = font_path('PRESENTATION_FONT_REGULAR', 'arial.ttf')
BOLD = font_path('PRESENTATION_FONT_BOLD', 'arialbd.ttf')
fonts = {False: fitz.Font(fontfile=FONT), True: fitz.Font(fontfile=BOLD)}
C = {'ink':'151B24','dark':'111720','panel':'1D2733','panel2':'25313E','cream':'F5F3ED','white':'FFFFFF','muted':'697583','mutedD':'AAB6C4','line':'DDDCD6','yellow':'FFD447','teal':'60CDBD','blue':'87ADEC','purple':'B6A0E6','coral':'F19B83'}
slides=[]
current=None

def rgb(c): return tuple(int(c[i:i+2],16)/255 for i in (0,2,4))
def add(e):
    current['elements'].append(e)
    return e

def rect(x,y,w,h,fill,stroke=None,sw=1): return add(dict(type='rect',x=x,y=y,w=w,h=h,fill=fill,stroke=stroke,sw=sw))
def circle(x,y,r,fill,stroke=None,sw=1): return add(dict(type='ellipse',x=x-r,y=y-r,w=2*r,h=2*r,fill=fill,stroke=stroke,sw=sw))
def line(x1,y1,x2,y2,color,width=1,dash=False): return add(dict(type='line',x1=x1,y1=y1,x2=x2,y2=y2,color=color,width=width,dash=dash))
def path(points,color,width=2):
    for a,b in zip(points,points[1:]): line(*a,*b,color,width)
def arrow(x1,y1,x2,y2,color,width=1.5,both=False):
    line(x1,y1,x2,y2,color,width)
    def tip(a,b):
        t=math.atan2(b[1]-a[1],b[0]-a[0]); l=6
        for d in (-.5,.5): line(b[0],b[1],b[0]-l*math.cos(t+d),b[1]-l*math.sin(t+d),color,width)
    tip((x1,y1),(x2,y2))
    if both: tip((x2,y2),(x1,y1))

def text(s,x,y,w,size=18,color=None,bold=False,align='left',lh=1.18,max_h=None):
    color=color or C['ink']; result=[]
    for para in str(s).split('\n'):
        if not para: result.append(''); continue
        words=para.split(' '); cur=''
        for word in words:
            cand=(cur+' '+word).strip()
            if cur and fonts[bold].text_length(cand,fontsize=size)>w*.97:
                result.append(cur); cur=word
            else: cur=cand
        result.append(cur)
    height=(len(result)-1)*size*lh+size*1.22
    if max_h is not None and height>max_h+.1:
        raise ValueError(f'Text too tall on {len(slides)}: {s!r} {height:.1f}>{max_h}')
    for i,t in enumerate(result):
        if fonts[bold].text_length(t,fontsize=size)>w+1: raise ValueError(f'Unbreakable text too wide: {t}')
        add(dict(type='text',text=t,x=x,y=y+i*size*lh,w=w,h=size*1.25,size=size,color=color,bold=bold,align=align))
    return y+height

def pill(label,x,y,w,color=C['yellow'],fg=C['ink'],size=11):
    rect(x,y,w,25,color)
    text(label,x+9,y+6,w-18,size,fg,True)

def number(n,x,y,color=C['yellow'],dark=False):
    circle(x,y,13,color)
    text(str(n),x-11,y-7,22,11,C['ink'],True,'center')

def new(title,source,subtitle=''):
    global current
    current={'background':C['cream'],'elements':[],'notes':'Основание: '+source,'title':title}
    slides.append(current)
    text('ПЛАН РАЗРАБОТКИ',48,25,680,10,C['muted'],True)
    text(title,48,53,864,31,C['ink'],True,max_h=43)
    if subtitle: text(subtitle,48,99,864,14,C['muted'],max_h=19)
    line(48,505,912,505,C['line'],.7)
    text(source,48,517,785,9,C['muted'])
    text(f'{len(slides):02d} / 10',848,515,64,10,C['muted'],True,'right')

def headers(labels,xs,ws,y):
    for label,x,w in zip(labels,xs,ws): text(label.upper(),x,y,w,10,C['muted'],True)

def module(title,source,col,interface,tasks,impl_body,use_body,dependency,start):
    new(title,'docs/contracts/'+source,interface)
    rect(48,136,517,276,C['white']); rect(48,136,5,276,col)
    text('ЗАДАЧИ',66,151,475,10,C['muted'],True)
    for i,(a,b) in enumerate(tasks):
        y=177+i*57
        text(a,66,y,478,16,bold=True)
        text(b,66,y+24,478,13,C['muted'],max_h=33)
    rect(581,136,331,276,'E7ECEF')
    text('РЕАЛИЗУЕТ',598,153,296,17,bold=True)
    text(impl_body,598,184,296,15,max_h=76)
    line(598,267,894,267,'C8D0D6',.8)
    text('ИСПОЛЬЗУЕТ ОТ ДРУГИХ',598,284,296,17,bold=True)
    text(use_body,598,315,296,15,max_h=81)
    text(dependency,48,430,864,14,bold=True,max_h=25)
    text(start,48,465,864,14,C['muted'],max_h=30)

# 01 — Solution and owners
new('Решение и распределение работы','docs/work_breakdown.md · docs/functional_scope.md',
    'Веб-приложение диспетчера: распределение заявок, маршруты и перепланирование.')
text('Backend: одно приложение, четыре внутренних модуля. Команда: 4 Go + 1 Frontend.',48,119,864,15,C['muted'])
headers(['Исполнитель','Часть решения','Основная ответственность'],[48,163,489],[105,310,423],160)
rows=[('Go-1','Планировщик','Назначения, расписание, причины неназначения',C['yellow']),
      ('Go-2','Данные и инфраструктура','Импорт, инженеры, API, хранение, сборка',C['teal']),
      ('Go-3','Геоданные','Координаты, дорожные матрицы, геометрия',C['blue']),
      ('Go-4','Управление планами','Расчёт, события, проверка, метрики и изменения',C['purple']),
      ('Frontend','Рабочее место диспетчера','Формы, карта, расписание, заявки, сравнение',C['coral'])]
for i,(owner,area,body,col) in enumerate(rows):
    y=187+i*53; rect(48,y,864,46,C['white']); rect(48,y,5,46,col)
    text(owner,62,y+13,93,14,bold=True); text(area,163,y+13,313,16,bold=True); text(body,489,y+14,410,13)
text('Основа: F1–F7. Дополнения D1–D3 — после совместной работы основных блоков.',48,472,864,14,C['muted'])

# 02 — Go-1
module('Go-1 — Планировщик','planner.md',C['yellow'],'Planner.Solve',[
    ('Назначение заявок','Выбрать инженера, порядок визитов и время работ.'),
    ('Ограничения','Учесть навыки, транспорт, окна, смены и время в пути.'),
    ('Два режима расчёта','Реализовать базовый алгоритм и оптимизацию.'),
    ('Неназначенные заявки','Вернуть причины; если размещение не найдено — указать это.')],
    'Planner.Solve → SolveResult: маршруты, расписание, неназначенные заявки и статус поиска.',
    'Нет вызовов модулей Go. Получает от Go-4 SolveRequest, включая TravelMatrix Go-3.',
    'Вызывается Go-4 через Planner.Solve.',
    'Для старта: несколько заявок, два инженера и фиксированная матрица.')

# 03 — Go-2
module('Go-2 — Данные и инфраструктура','data.md',C['teal'],'DataStore + HTTP API',[
    ('Данные и импорт','Заявки, инженеры, навыки, смены, транспорт и нормативы.'),
    ('Подготовка снимка','Проверка, редактирование, версии данных; геокодирование через Go-3.'),
    ('HTTP и хранение','Запуски расчёта, история планов, атомарное сохранение и повторы команд.'),
    ('Сборка приложения','Подключение модулей, конфигурация и запуск.')],
    'DataStore + HTTP /api/v1: импорт, снимки, инженеры, Run, Plan и сохранение результата.',
    'PlanService.Build / Replan (Go-4); GeoService.Geocode (Go-3).',
    'Frontend использует HTTP /api/v1; Go-2 собирает приложение.',
    'Для старта: заглушка PlanService и подготовленные координаты.')

# 04 — Go-3
module('Go-3 — Геоданные','geo.md',C['blue'],'GeoService',[
    ('Geocode','Получить координаты адресов и ошибки распознавания.'),
    ('BuildMatrix','Время и расстояние по дорогам: автомобиль и пеший профиль.'),
    ('BuildRoutes','Геометрия уже выбранных участков маршрута; кэширование.'),
    ('PositionAt','Модельное положение инженера в пути на момент события.')],
    'GeoService: Geocode, BuildMatrix, BuildRoutes, PositionAt.',
    'Не вызывает Go-модули; внешняя геосистема — внутренняя зависимость Go-3.',
    'Вызывается Go-2 и Go-4 по контракту GeoService.',
    'Для старта: подготовленные адреса, точки и последовательности участков.')

# 05 — Go-4
module('Go-4 — Управление планами','plans.md',C['purple'],'PlanService.Build / Replan',[
    ('Обычный расчёт','Получить снимок и матрицу; вызвать оба режима Go-1.'),
    ('Обработка событий','Срочная заявка, отмена заявки, недоступность инженера.'),
    ('Перепланирование','Сохранить выполненное и начатое; пересчитать будущую часть.'),
    ('Проверка и сравнение','Проверить ограничения, добавить геометрию, метрики и изменения.')],
    'PlanService.Build / Replan → PlanResult: проверенный план, метрики и изменения.',
    'DataStore.GetSnapshot / GetPlan (Go-2); все 4 метода GeoService (Go-3); Planner.Solve (Go-1).',
    'Вызывается Go-2; Go-4 не сохраняет план напрямую.',
    'Для старта: готовые снимки и планы, заглушки географии и планировщика.')

# 06 — Frontend
module('Frontend — Рабочее место диспетчера','frontend.md',C['coral'],'HTTP API /api/v1',[
    ('Данные и инженеры','Загрузка CSV / демо-набора, ошибки данных, настройка инженеров.'),
    ('Рабочий экран','Карта, расписание, карточки заявок, связанный выбор и фильтры.'),
    ('Результат расчёта','Неназначенные заявки, причины и сравнение метрик.'),
    ('События','Формы трёх событий и отображение изменений нового плана.')],
    'Интерфейс диспетчера: загрузка данных, карта, расписание, заявки, события и метрики.',
    'HTTP /api/v1 от Go-2: команды и данные инженеров, Run, Plan, Snapshot и ошибки.',
    'Расчёт: команда → run_id → опрос Run → загрузка Plan и Snapshot.',
    'Для старта: согласованные mock-ответы, включая ошибки и конфликты версий.')

# 07 — Module contracts
new('Контракты между частями','docs/contracts/*.md','Внутри backend — Go-интерфейсы; между Frontend и backend — HTTP/JSON.')
headers(['Кто вызывает','Чью часть','Контракт и назначение'],[48,234,397],[175,151,515],134)
links=[('Frontend','Go-2','HTTP /api/v1: данные, расчёт, события, Run и Plan'),
       ('Go-2','Go-4','PlanService.Build / Replan → PlanResult'),
       ('Go-4','Go-2','DataStore.GetSnapshot / GetPlan → снимок и план'),
       ('Go-2, Go-4','Go-3','GeoService: адреса, матрицы, маршруты, положение'),
       ('Go-4','Go-1','Planner.Solve(SolveRequest) → SolveResult')]
for i,(a,b,c) in enumerate(links):
    y=165+i*52; rect(48,y,864,45,C['white']); text(a,61,y+13,166,15,bold=True); text(b,247,y+13,138,15,bold=True); text(c,410,y+14,487,13)
rect(48,436,864,57,'E7ECEF')
text('Сохранение: Go-2 получает PlanResult от Go-4 и вызывает DataStore.CommitPlan.',63,448,830,14,bold=True)
text('Go-4 проверяет и возвращает результат; Go-2 сохраняет новую версию.',63,472,830,13,C['muted'])

# 08 — Parallel development
new('Параллельная разработка','docs/work_breakdown.md · docs/contracts/*.md','Перед началом: общие модели, единицы измерения, интерфейсы и примеры входов / выходов.')
headers(['Исполнитель','С чем начинает независимо','Что подключает при интеграции'],[48,180,552],[121,358,360],139)
parallel=[('Go-1','Заявки, инженеры, фиксированная матрица','Подготовленную задачу от Go-4'),
          ('Go-2','Заглушка PlanService, готовые координаты','PlanService Go-4 и Geocode Go-3'),
          ('Go-3','Адреса, точки и участки для расчётов','Вызовы Go-2 и Go-4'),
          ('Go-4','Готовые планы, заглушки зависимостей','DataStore, GeoService и Planner'),
          ('Frontend','Согласованные mock-ответы API','Реальный HTTP API Go-2')]
for i,(a,b,c) in enumerate(parallel):
    y=173+i*55; rect(48,y,864,48,C['white']); text(a,61,y+15,112,14,bold=True); text(b,193,y+10,340,13,max_h=35); text(c,565,y+10,332,13,max_h=35)
text('Go-1 и Go-4 совместно согласуют правила допустимости плана.',48,472,864,15,bold=True)

# 09 — Integration
new('Интеграция и ответственность','docs/work_breakdown.md','Рано собираем полный сценарий на базовом алгоритме, затем подключаем оптимизацию и события.')
headers(['Порядок','Состав работ','Ответственные'],[48,161,645],[100,469,267],138)
stages=[('1','Общие типы, интерфейсы и примеры данных','Все разработчики'),
        ('2','Данные → базовый расчёт → проверка и метрики → карта и расписание','Go-2 — сборка; каждый — свой модуль'),
        ('3','Оптимизация и сравнение с базовым результатом','Go-1 — расчёт; Go-4 — проверка; Frontend — показ'),
        ('4','Три события → новый план → изменения и метрики','Go-4 — процесс; Go-2 — сохранение; Frontend — формы')]
for i,(a,b,c) in enumerate(stages):
    y=173+i*67; rect(48,y,864,59,C['white']); text(a,65,y+16,70,22,bold=True); text(b,174,y+11,446,14,max_h=41); text(c,658,y+10,242,13,max_h=44)
rect(48,455,864,37,'E7ECEF'); text('Go-2 — сборка приложения. Go-4 — сквозная проверка. Каждый исправляет свою часть.',62,465,836,13,bold=True)

# 10 — Later work
new('Дополнения после основы','docs/work_breakdown.md · docs/functional_scope.md','Начинаем после совместной работы блоков F1–F7.')
extras=[('D1','Обеденный перерыв','Go-1',C['yellow'],[
    'Go-1: учёт перерыва в расписании.',
    'Go-2: параметры и хранение.',
    'Go-4: проверка и перепланирование.',
    'Frontend: ввод и отображение.']),
    ('D2','Ручное переназначение\nи закрепление','Go-4',C['purple'],[
    'Go-4: процесс изменения и проверка.',
    'Go-1: закрепления в алгоритме.',
    'Go-2: API и хранение.',
    'Frontend: форма и конфликты.']),
    ('D3','Учёт оборудования','Go-1',C['teal'],[
    'Go-1: ограничения по оборудованию.',
    'Go-2: запасы и потребности.',
    'Go-4: проверка и остатки при пересчёте.',
    'Frontend: ввод и отображение.'])]
for i,(code,title,lead,col,tasks) in enumerate(extras):
    x=48+i*292; rect(x,147,280,346,C['white']); rect(x,147,280,4,col)
    text(code,x+18,168,244,13,C['muted'],True); text(title,x+18,195,244,19,bold=True,max_h=60)
    text('Ведущий: '+lead,x+18,261,244,15,bold=True)
    for j,t in enumerate(tasks): text(t,x+18,305+j*43,244,13,max_h=34)

# Validate scene bounds and render.
for i,s in enumerate(slides,1):
    for e in s['elements']:
        if e['type']=='line':
            vals=[e['x1'],e['x2']]; ys=[e['y1'],e['y2']]
            if min(vals)<-.1 or max(vals)>W+.1 or min(ys)<-.1 or max(ys)>H+.1: raise ValueError(f'Line outside slide {i}')
        else:
            if min(e['x'],e['y'])<-.1 or e['x']+e['w']>W+.1 or e['y']+e['h']>H+.1: raise ValueError(f'Element outside slide {i}: {e}')

(ROOT/'source'/'deck.json').write_text(json.dumps({'width':W,'height':H,'slides':slides},ensure_ascii=False,indent=2))
doc=fitz.open()
for i,s in enumerate(slides,1):
    page=doc.new_page(width=W,height=H)
    page.insert_font(fontname='Arial',fontfile=FONT); page.insert_font(fontname='ArialB',fontfile=BOLD)
    page.draw_rect(page.rect,fill=rgb(s['background']),color=rgb(s['background']))
    for e in s['elements']:
        typ=e['type']
        if typ in ('rect','ellipse'):
            r=fitz.Rect(e['x'],e['y'],e['x']+e['w'],e['y']+e['h'])
            fn=page.draw_rect if typ=='rect' else page.draw_oval
            fn(r,fill=rgb(e['fill']),color=rgb(e['stroke']) if e.get('stroke') else None,width=e['sw'])
        elif typ=='line':
            page.draw_line((e['x1'],e['y1']),(e['x2'],e['y2']),color=rgb(e['color']),width=e['width'],dashes='[4 3] 0' if e['dash'] else None)
        else:
            font=fonts[bool(e['bold'])]; tw=font.text_length(e['text'],fontsize=e['size'])
            x=e['x']+(e['w']-tw if e['align']=='right' else (e['w']-tw)/2 if e['align']=='center' else 0)
            page.insert_text((x,e['y']+e['size']*.91),e['text'],fontname='ArialB' if e['bold'] else 'Arial',fontsize=e['size'],color=rgb(e['color']))
    pix=page.get_pixmap(matrix=fitz.Matrix(1.5,1.5),alpha=False)
    pix.save(str(ROOT/'preview'/f'{i:02d}.png'))
doc.set_metadata({'title':'План разработки — распределение работы и контракты','author':'Команда проекта','subject':'Концепция, архитектура, распределение ответственности и контракты'})
doc.subset_fonts()
doc.save(str(ROOT/'solution.pdf'),garbage=4,deflate=True)
# Contact sheet of all slides.
thumb_w,thumb_h=480,270
sheet=Image.new('RGB',(thumb_w*2+36, (thumb_h+32)*math.ceil(len(slides)/2)+18),'#DDE1E3')
for i in range(len(slides)):
    im=Image.open(ROOT/'preview'/f'{i+1:02d}.png').convert('RGB').resize((thumb_w,thumb_h),Image.Resampling.LANCZOS)
    x=12+(i%2)*(thumb_w+12); y=12+(i//2)*(thumb_h+32)
    sheet.paste(im,(x,y))
sheet.save(ROOT/'preview'/'all_slides.jpg',quality=92)
print(f'Created {len(slides)} slides, PDF, scene JSON and previews in {ROOT}')
