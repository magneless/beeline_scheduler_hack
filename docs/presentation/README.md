# Презентация решения и распределения работы

Готовые файлы: [PowerPoint](solution.pptx) и [PDF](solution.pdf).

Презентация отражает распределение и контракты до Q&A 2. Новые ответы и необходимые уточнения модели зафиксированы в [анализе от 19.09.2026](../qa_2/QA_2_impact.md); перепланирование остаётся в основе. Эта фиксация Q&A не пересобирает слайды.

Исходный текст и оформление находятся в `source/build_deck.py`. Он создаёт PDF, `source/deck.json` и изображения в `preview/`. Скрипт `source/build_pptx.cjs` создаёт редактируемый PowerPoint из того же JSON.

Для пересборки нужны Python 3 с `PyMuPDF` и `Pillow`, Node.js с `pptxgenjs` 4.0.1, а также Arial (обычный и полужирный). Команды из корня репозитория:

```bash
python3 -m pip install PyMuPDF Pillow
npm install --no-save --package-lock=false pptxgenjs@4.0.1
python3 docs/presentation/source/build_deck.py
node docs/presentation/source/build_pptx.cjs
```

Arial автоматически ищется в стандартной папке Windows, в том числе через WSL. На другой системе задайте пути к установленным файлам шрифта через `PRESENTATION_FONT_REGULAR` и `PRESENTATION_FONT_BOLD`. Для внешней установки PptxGenJS можно указать каталог модуля через `PPTXGENJS_PATH`.

Содержимое основано на [распределении работы](../work_breakdown.md) и [контрактах](../contracts/common.md).
