using System.Globalization;
using System.Windows;
using System.Windows.Data;
using System.Windows.Media;
using Brush = System.Windows.Media.Brush;
using Brushes = System.Windows.Media.Brushes;
using Color = System.Windows.Media.Color;
using FontFamily = System.Windows.Media.FontFamily;
using Pen = System.Windows.Media.Pen;
using Point = System.Windows.Point;
using Size = System.Windows.Size;

namespace FlowLight.Client;

static class Palette
{
    public static Brush Get(string key) =>
        System.Windows.Application.Current?.TryFindResource(key) as Brush ?? Brushes.Gray;

    public static Color ColorOf(string key) => Get(key) is SolidColorBrush solid ? solid.Color : Colors.Gray;

    public static Color WithAlpha(Color color, byte alpha) => Color.FromArgb(alpha, color.R, color.G, color.B);

    public static SolidColorBrush Solid(Color color)
    {
        var brush = new SolidColorBrush(color);
        brush.Freeze();
        return brush;
    }

    // FNV-1a: the same game or region always gets the same colours.
    public static uint Hash(string? text)
    {
        var hash = 2166136261u;
        foreach (var ch in text ?? "")
        {
            hash = (hash ^ ch) * 16777619u;
        }
        return hash;
    }

    // Hues that stay clear at a dark cover's lightness; the olive and mustard
    // band in between turns muddy, so covers never land there.
    static readonly double[] CoverHues = [222, 340, 168, 22, 252, 140, 312, 192, 6];

    public static double CoverHue(uint seed) => CoverHues[seed % (uint)CoverHues.Length] + (seed >> 8) % 11 - 5.0;

    // Games take hues in catalogue order; consecutive entries of the list are
    // far apart, so neighbouring cards never share a colour family.
    public static double SteppedHue(int index) => CoverHues[index % CoverHues.Length];

    public static Color Hsl(double h, double s, double l, byte alpha = 255)
    {
        h = ((h % 360) + 360) % 360;
        var c = (1 - Math.Abs(2 * l - 1)) * s;
        var x = c * (1 - Math.Abs(h / 60 % 2 - 1));
        var m = l - c / 2;
        var (r, g, b) = h switch
        {
            < 60 => (c, x, 0.0),
            < 120 => (x, c, 0.0),
            < 180 => (0.0, c, x),
            < 240 => (0.0, x, c),
            < 300 => (x, 0.0, c),
            _ => (c, 0.0, x),
        };
        return Color.FromArgb(alpha, (byte)((r + m) * 255), (byte)((g + m) * 255), (byte)((b + m) * 255));
    }
}

// An outline icon drawn from a 24 x 24 geometry, so every icon shares one
// stroke weight whatever size it is shown at.
public sealed class Glyph : FrameworkElement
{
    public static readonly DependencyProperty DataProperty = DependencyProperty.Register(
        nameof(Data), typeof(Geometry), typeof(Glyph), new FrameworkPropertyMetadata(null, FrameworkPropertyMetadataOptions.AffectsRender));
    public static readonly DependencyProperty InkProperty = DependencyProperty.Register(
        nameof(Ink), typeof(Brush), typeof(Glyph), new FrameworkPropertyMetadata(null, FrameworkPropertyMetadataOptions.AffectsRender));
    public static readonly DependencyProperty IconSizeProperty = DependencyProperty.Register(
        nameof(IconSize), typeof(double), typeof(Glyph), new FrameworkPropertyMetadata(20.0, FrameworkPropertyMetadataOptions.AffectsMeasure | FrameworkPropertyMetadataOptions.AffectsRender));
    public static readonly DependencyProperty FilledProperty = DependencyProperty.Register(
        nameof(Filled), typeof(bool), typeof(Glyph), new FrameworkPropertyMetadata(false, FrameworkPropertyMetadataOptions.AffectsRender));
    public static readonly DependencyProperty ThicknessProperty = DependencyProperty.Register(
        nameof(Thickness), typeof(double), typeof(Glyph), new FrameworkPropertyMetadata(1.8, FrameworkPropertyMetadataOptions.AffectsRender));

    public Geometry? Data { get => (Geometry?)GetValue(DataProperty); set => SetValue(DataProperty, value); }
    public Brush? Ink { get => (Brush?)GetValue(InkProperty); set => SetValue(InkProperty, value); }
    public double IconSize { get => (double)GetValue(IconSizeProperty); set => SetValue(IconSizeProperty, value); }
    public bool Filled { get => (bool)GetValue(FilledProperty); set => SetValue(FilledProperty, value); }
    public double Thickness { get => (double)GetValue(ThicknessProperty); set => SetValue(ThicknessProperty, value); }

    protected override Size MeasureOverride(Size availableSize) => new(IconSize, IconSize);

    protected override void OnRender(DrawingContext dc)
    {
        if (Data is null)
        {
            return;
        }
        var ink = Ink ?? Palette.Get("TextSecondary");
        var scale = IconSize / 24;
        dc.PushTransform(new ScaleTransform(scale, scale));
        if (Filled)
        {
            dc.DrawGeometry(ink, null, Data);
        }
        else
        {
            var pen = new Pen(ink, Thickness)
            {
                StartLineCap = PenLineCap.Round,
                EndLineCap = PenLineCap.Round,
                LineJoin = PenLineJoin.Round,
            };
            dc.DrawGeometry(null, pen, Data);
        }
        dc.Pop();
    }
}

// Four rising bars; Level of them are lit in Ink.
public sealed class SignalBars : FrameworkElement
{
    public static readonly DependencyProperty LevelProperty = DependencyProperty.Register(
        nameof(Level), typeof(int), typeof(SignalBars), new FrameworkPropertyMetadata(0, FrameworkPropertyMetadataOptions.AffectsRender));
    public static readonly DependencyProperty InkProperty = DependencyProperty.Register(
        nameof(Ink), typeof(Brush), typeof(SignalBars), new FrameworkPropertyMetadata(null, FrameworkPropertyMetadataOptions.AffectsRender));

    public int Level { get => (int)GetValue(LevelProperty); set => SetValue(LevelProperty, value); }
    public Brush? Ink { get => (Brush?)GetValue(InkProperty); set => SetValue(InkProperty, value); }

    protected override Size MeasureOverride(Size availableSize) => new(22, 18);

    protected override void OnRender(DrawingContext dc)
    {
        var lit = Ink ?? Palette.Get("Good");
        var dim = Palette.Get("BorderStrong");
        for (var i = 0; i < 4; i++)
        {
            var height = 6 + i * 4;
            var rect = new Rect(i * 5.5, 18 - height, 3.5, height);
            dc.DrawRoundedRectangle(i < Level ? lit : dim, null, rect, 1.2, 1.2);
        }
    }
}

// Generated cover art: each game gets its own hue, a soft light source and
// a few angled light streaks (the brand's "flowing light"), with the first
// character of its name set large and faint. No game artwork is bundled.
public sealed class GameCover : FrameworkElement
{
    public static readonly DependencyProperty SeedProperty = DependencyProperty.Register(
        nameof(Seed), typeof(string), typeof(GameCover), new FrameworkPropertyMetadata("", FrameworkPropertyMetadataOptions.AffectsRender));
    public static readonly DependencyProperty MonogramProperty = DependencyProperty.Register(
        nameof(Monogram), typeof(string), typeof(GameCover), new FrameworkPropertyMetadata("", FrameworkPropertyMetadataOptions.AffectsRender));
    public static readonly DependencyProperty RadiusProperty = DependencyProperty.Register(
        nameof(Radius), typeof(double), typeof(GameCover), new FrameworkPropertyMetadata(10.0, FrameworkPropertyMetadataOptions.AffectsRender));
    public static readonly DependencyProperty ShadeProperty = DependencyProperty.Register(
        nameof(Shade), typeof(bool), typeof(GameCover), new FrameworkPropertyMetadata(false, FrameworkPropertyMetadataOptions.AffectsRender));
    public static readonly DependencyProperty HueProperty = DependencyProperty.Register(
        nameof(Hue), typeof(double), typeof(GameCover), new FrameworkPropertyMetadata(double.NaN, FrameworkPropertyMetadataOptions.AffectsRender));
    public static readonly DependencyProperty NightProperty = DependencyProperty.Register(
        nameof(Night), typeof(bool), typeof(GameCover), new FrameworkPropertyMetadata(false, FrameworkPropertyMetadataOptions.AffectsRender));

    public string Seed { get => (string)GetValue(SeedProperty); set => SetValue(SeedProperty, value); }
    public string Monogram { get => (string)GetValue(MonogramProperty); set => SetValue(MonogramProperty, value); }
    public double Radius { get => (double)GetValue(RadiusProperty); set => SetValue(RadiusProperty, value); }
    public bool Shade { get => (bool)GetValue(ShadeProperty); set => SetValue(ShadeProperty, value); }
    public double Hue { get => (double)GetValue(HueProperty); set => SetValue(HueProperty, value); }
    // Dark ground with brighter streaks: the brand panel rather than a game.
    public bool Night { get => (bool)GetValue(NightProperty); set => SetValue(NightProperty, value); }

    static readonly FontFamily MonogramFace = new("Bahnschrift, Microsoft YaHei UI");

    protected override void OnRender(DrawingContext dc)
    {
        double w = ActualWidth, h = ActualHeight;
        if (w < 2 || h < 2)
        {
            return;
        }
        var bounds = new Rect(0, 0, w, h);
        dc.PushClip(new RectangleGeometry(bounds, Radius, Radius));

        var seed = Palette.Hash(Seed);
        var hue = double.IsNaN(Hue) ? Palette.CoverHue(seed) : Hue;
        var night = Night;
        dc.DrawRectangle(new LinearGradientBrush(
            night ? Palette.Hsl(hue + 6, 0.32, 0.17) : Palette.Hsl(hue, 0.52, 0.33),
            night ? Palette.Hsl(hue + 20, 0.22, 0.06) : Palette.Hsl(hue + 28, 0.58, 0.11),
            new Point(0, 0), new Point(1, 1)), null, bounds);
        var glowColor = night ? Palette.Hsl(hue, 0.9, 0.55, 120) : Palette.Hsl(hue - 12, 0.8, 0.64, 150);
        dc.DrawRectangle(new RadialGradientBrush(glowColor, Palette.WithAlpha(glowColor, 0))
        {
            Center = new Point(0.8, 0.18),
            GradientOrigin = new Point(0.8, 0.18),
            RadiusX = 0.62,
            RadiusY = 0.95,
        }, null, bounds);

        var random = new Random((int)(seed & 0x7FFFFFFF));
        dc.PushTransform(new RotateTransform(-22, w / 2, h / 2));
        for (var i = 0; i < (night ? 12 : 7); i++)
        {
            var thickness = Math.Max(1.2, h * (0.006 + random.NextDouble() * 0.02));
            var length = w * (0.3 + random.NextDouble() * 0.55);
            var x = -w * 0.25 + random.NextDouble() * w * 1.1;
            var y = -h * 0.15 + random.NextDouble() * h * 1.25;
            var light = night
                ? Palette.Hsl(hue, 0.95, 0.7, (byte)(70 + random.Next(150)))
                : Palette.Hsl(hue - 24, 0.95, 0.84, (byte)(45 + random.Next(120)));
            var streak = new LinearGradientBrush { StartPoint = new Point(0, 0.5), EndPoint = new Point(1, 0.5) };
            streak.GradientStops.Add(new GradientStop(Palette.WithAlpha(light, 0), 0));
            streak.GradientStops.Add(new GradientStop(light, 0.72));
            streak.GradientStops.Add(new GradientStop(Palette.WithAlpha(light, 0), 1));
            dc.DrawRoundedRectangle(streak, null, new Rect(x, y, length, thickness), thickness / 2, thickness / 2);
        }
        dc.Pop();

        if (!string.IsNullOrEmpty(Monogram))
        {
            var text = new FormattedText(
                Monogram,
                CultureInfo.CurrentUICulture,
                System.Windows.FlowDirection.LeftToRight,
                new Typeface(MonogramFace, FontStyles.Normal, FontWeights.Bold, FontStretches.Normal),
                h * 0.92,
                Palette.Solid(Color.FromArgb(38, 255, 255, 255)),
                VisualTreeHelper.GetDpi(this).PixelsPerDip);
            dc.DrawText(text, new Point(w - text.Width * 0.9, h - text.Height * 0.84));
        }

        if (Shade)
        {
            var shade = new LinearGradientBrush { StartPoint = new Point(0.5, 0), EndPoint = new Point(0.5, 1) };
            shade.GradientStops.Add(new GradientStop(Color.FromArgb(0, 10, 9, 12), 0.25));
            shade.GradientStops.Add(new GradientStop(Color.FromArgb(215, 10, 9, 12), 1));
            dc.DrawRectangle(shade, null, bounds);
        }
        dc.DrawRoundedRectangle(null, new Pen(Palette.Solid(Color.FromArgb(30, 255, 255, 255)), 1), new Rect(0.5, 0.5, w - 1, h - 1), Radius, Radius);
        dc.Pop();
    }
}

// Latency of each tunnel probe in this session, plotted against time with
// a scale of round numbers. Only measured samples are drawn.
public sealed class LatencyChart : FrameworkElement
{
    public static readonly DependencyProperty InkProperty = DependencyProperty.Register(
        nameof(Ink), typeof(Brush), typeof(LatencyChart), new FrameworkPropertyMetadata(null, FrameworkPropertyMetadataOptions.AffectsRender));

    public Brush? Ink { get => (Brush?)GetValue(InkProperty); set => SetValue(InkProperty, value); }

    readonly List<(DateTime At, double Ms)> samples = new();

    public void SetSamples(IEnumerable<(DateTime At, double Ms)> values)
    {
        samples.Clear();
        samples.AddRange(values);
        InvalidateVisual();
    }

    static readonly FontFamily LabelFace = new("Bahnschrift, Microsoft YaHei UI");

    static string Ago(double seconds) =>
        seconds < 60 ? $"{seconds:0} 秒前" : seconds < 3600 ? $"{seconds / 60:0} 分钟前" : $"{seconds / 3600:0.#} 小时前";

    FormattedText Label(string text, Brush brush, double size = 10.5) => new(
        text,
        CultureInfo.CurrentUICulture,
        System.Windows.FlowDirection.LeftToRight,
        new Typeface(LabelFace, FontStyles.Normal, FontWeights.Normal, FontStretches.Normal),
        size,
        brush,
        VisualTreeHelper.GetDpi(this).PixelsPerDip);

    protected override void OnRender(DrawingContext dc)
    {
        double w = ActualWidth, h = ActualHeight;
        if (w < 40 || h < 40)
        {
            return;
        }
        var muted = Palette.Get("TextTertiary");
        var grid = new Pen(Palette.Get("Border"), 1);
        var plot = new Rect(34, 8, w - 34 - 10, h - 8 - 22);

        var max = samples.Count == 0 ? 0 : samples.Max(s => s.Ms);
        var step = new[] { 10.0, 20, 25, 50, 100, 200, 250, 500, 1000 }.First(s => s * 4 >= Math.Max(max * 1.15, 40));
        var top = step * 4;
        for (var k = 0; k <= 4; k++)
        {
            var y = Math.Round(plot.Bottom - plot.Height * k / 4) + 0.5;
            dc.DrawLine(grid, new Point(plot.Left, y), new Point(plot.Right, y));
            var label = Label((step * k).ToString("0"), muted);
            dc.DrawText(label, new Point(plot.Left - 8 - label.Width, y - label.Height / 2));
        }

        if (samples.Count == 0)
        {
            var waiting = Label("等待首次检测", Palette.Get("TextSecondary"), 12);
            dc.DrawText(waiting, new Point(plot.Left + (plot.Width - waiting.Width) / 2, plot.Top + (plot.Height - waiting.Height) / 2));
            return;
        }

        var ink = Ink ?? Palette.Get("Good");
        var color = ink is SolidColorBrush solid ? solid.Color : Colors.White;
        var first = samples[0].At;
        var span = (samples[^1].At - first).TotalSeconds;
        Point At(int i) => new(
            samples.Count == 1 || span <= 0 ? plot.Right : plot.Left + plot.Width * (samples[i].At - first).TotalSeconds / span,
            plot.Bottom - plot.Height * Math.Min(samples[i].Ms, top) / top);

        if (samples.Count > 1)
        {
            var area = new StreamGeometry();
            var line = new StreamGeometry();
            using (var ctx = area.Open())
            {
                ctx.BeginFigure(new Point(At(0).X, plot.Bottom), true, true);
                for (var i = 0; i < samples.Count; i++)
                {
                    ctx.LineTo(At(i), true, true);
                }
                ctx.LineTo(new Point(At(samples.Count - 1).X, plot.Bottom), true, true);
            }
            using (var ctx = line.Open())
            {
                ctx.BeginFigure(At(0), false, false);
                for (var i = 1; i < samples.Count; i++)
                {
                    ctx.LineTo(At(i), true, true);
                }
            }
            var fill = new LinearGradientBrush { StartPoint = new Point(0.5, 0), EndPoint = new Point(0.5, 1) };
            fill.GradientStops.Add(new GradientStop(Palette.WithAlpha(color, 90), 0));
            fill.GradientStops.Add(new GradientStop(Palette.WithAlpha(color, 0), 1));
            dc.DrawGeometry(fill, null, area);
            dc.DrawGeometry(null, new Pen(ink, 2) { LineJoin = PenLineJoin.Round }, line);
            if (samples.Count <= 40)
            {
                for (var i = 0; i < samples.Count - 1; i++)
                {
                    dc.DrawEllipse(Palette.Get("Surface"), new Pen(ink, 1.5), At(i), 2.5, 2.5);
                }
            }
        }

        var last = At(samples.Count - 1);
        dc.DrawEllipse(Palette.Solid(Palette.WithAlpha(color, 60)), null, last, 9, 9);
        dc.DrawEllipse(ink, null, last, 4.5, 4.5);
        var value = Label(samples[^1].Ms.ToString("0") + " ms", ink, 12);
        dc.DrawText(value, new Point(Math.Max(plot.Left, last.X - value.Width - 12), Math.Max(0, last.Y - value.Height - 8)));

        var endLabel = Label("最新", muted);
        dc.DrawText(endLabel, new Point(plot.Right - endLabel.Width, plot.Bottom + 6));
        if (samples.Count > 1)
        {
            dc.DrawText(Label(Ago(span), muted), new Point(plot.Left, plot.Bottom + 6));
        }
    }
}

// Visible when the two bound identifiers are equal and present; used for the
// "running" and "last used" badges on game covers.
// True when a session runs for a game other than this one; such cards dim.
public sealed class OtherRunning : IMultiValueConverter
{
    public object Convert(object[] values, Type targetType, object parameter, CultureInfo culture) =>
        values.Length == 2 && values[0] is string id && values[1] is string running && running.Length > 0 && id != running;

    public object[] ConvertBack(object value, Type[] targetTypes, object parameter, CultureInfo culture) =>
        throw new NotSupportedException();
}

public sealed class MatchVisibility : IMultiValueConverter
{
    public object Convert(object[] values, Type targetType, object parameter, CultureInfo culture) =>
        values.Length == 2 && values[0] is string a && values[1] is string b && a.Length > 0 && a == b
            ? Visibility.Visible
            : Visibility.Collapsed;

    public object[] ConvertBack(object value, Type[] targetTypes, object parameter, CultureInfo culture) =>
        throw new NotSupportedException();
}
