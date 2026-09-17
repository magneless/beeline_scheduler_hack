// Editable PowerPoint from the same scene used for the PDF and previews.
const fs = require('fs');
const path = require('path');
const libraryPath = process.env.PPTXGENJS_PATH || 'pptxgenjs';
const PptxGenJS = require(libraryPath);
const deck = JSON.parse(fs.readFileSync(path.join(__dirname, 'deck.json'), 'utf8'));
const pptx = new PptxGenJS();
pptx.defineLayout({ name: 'CUSTOM', width: deck.width / 72, height: deck.height / 72 });
pptx.layout = 'CUSTOM';
pptx.author = 'Команда проекта';
pptx.subject = 'Задачи исполнителей, контракты и порядок интеграции';
pptx.title = 'Распределение работы и контракты';
pptx.company = 'Команда ЛЦТ';
pptx.lang = 'ru-RU';
pptx.theme = { headFontFace: 'Arial', bodyFontFace: 'Arial', lang: 'ru-RU' };

for (const scene of deck.slides) {
  const slide = pptx.addSlide();
  slide.background = { color: scene.background };
  slide.addNotes(scene.notes);
  for (const e of scene.elements) {
    if (e.type === 'text') {
      slide.addText(e.text, {
        x: e.x / 72, y: e.y / 72, w: e.w / 72, h: e.h / 72,
        fontFace: 'Arial', fontSize: e.size, bold: e.bold, color: e.color,
        align: e.align, valign: 'top', margin: 0, breakLine: false,
        paraSpaceAfterPt: 0, paraSpaceBeforePt: 0,
        fit: 'none', wrap: false, lang: 'ru-RU',
      });
    } else if (e.type === 'line') {
      slide.addShape(pptx.ShapeType.line, {
        x: Math.min(e.x1, e.x2) / 72, y: Math.min(e.y1, e.y2) / 72,
        w: Math.abs(e.x2 - e.x1) / 72, h: Math.abs(e.y2 - e.y1) / 72,
        flipV: (e.x2 - e.x1) * (e.y2 - e.y1) < 0,
        line: { color: e.color, width: e.width, dashType: e.dash ? 'dash' : 'solid', beginArrowType: 'none', endArrowType: 'none' },
      });
    } else {
      slide.addShape(e.type === 'ellipse' ? pptx.ShapeType.ellipse : pptx.ShapeType.rect, {
        x: e.x / 72, y: e.y / 72, w: e.w / 72, h: e.h / 72,
        fill: { color: e.fill },
        line: e.stroke ? { color: e.stroke, width: e.sw } : { color: e.fill, transparency: 100 },
      });
    }
  }
}
pptx.writeFile({ fileName: path.join(__dirname, '..', 'solution.pptx'), compression: true })
  .then(() => console.log(`Created editable PowerPoint: ${deck.slides.length} slides`));
