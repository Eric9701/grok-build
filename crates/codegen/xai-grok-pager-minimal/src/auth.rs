//! Minimal-mode sign-in / folder-trust rendering for the live region.
//!
//! Minimal has no welcome screen, so before any agent session exists the live region shows the sign-in flow itself.
//! [`draw_live`](super::live::draw_live) maps [`AuthState`] and [`TrustState`] to a [`MinimalAuthHint`] and renders it via [`render_auth`].

use std::path::PathBuf;

use ratatui::buffer::Buffer;
use ratatui::layout::Rect;
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};

use xai_grok_pager::app::app_view::{AuthMode, AuthState, CredentialField, CredentialForm, TrustState};
use xai_grok_pager::theme::Theme;

/// What the minimal live region shows when there is no active agent yet.
/// Computed before the draw closure so the closure can own it.
pub(super) enum MinimalAuthHint {
    /// Interactive sign-in underway: show the URL (when known) and the device code (when the URL carries one).
    /// Covers device flow and the external command flow, where the provider opens its own browser and `url` may be `None`.
    SigningIn {
        url: Option<String>,
        code: Option<String>,
    },
    /// The last sign-in attempt failed; show the error.
    Failed(String),
    /// Authenticated, but the cwd has untrusted repo-local config: ask before creating a session.
    /// Input (y/Enter trust, n/Esc quit) is handled by the welcome interceptor in `AppView::handle_input`; this is render-only.
    TrustFolder { workspace: PathBuf },
    /// Authenticated and trusted; the session is being created (brief transient).
    Starting,
    /// No-browser login. The password is already masked; the live region only paints this snapshot.
    Credentials {
        issuer: String,
        email: String,
        email_cursor: usize,
        password_mask: String,
        password_cursor: usize,
        machine_code: String,
        machine_cursor: usize,
        focus: CredentialField,
        error: Option<String>,
    },
    /// Password grant is in flight. No browser and no device code.
    SigningInPassword,
}

/// Map the app's auth and trust state to what the no-agent live region should show.
/// Mirrors the welcome screen's gate order: trust is only offered after auth is `Done`, when the user has access and is not ZDR-blocked.
/// Those gates already block sessions, and the input interceptor only answers trust under the same conditions.
pub(super) fn minimal_auth_hint(
    auth: &AuthState,
    trust: &TrustState,
    has_access: bool,
    is_zdr_blocked: bool,
) -> MinimalAuthHint {
    match auth {
        AuthState::Credentials(form) => credential_hint(form),
        AuthState::Authenticating {
            mode: AuthMode::Password,
            ..
        } => MinimalAuthHint::SigningInPassword,
        AuthState::Authenticating { auth_url, .. } => MinimalAuthHint::SigningIn {
            url: auth_url.clone(),
            code: auth_url
                .as_deref()
                .and_then(device_user_code)
                .map(str::to_owned),
        },
        AuthState::Pending { error: Some(err) } => MinimalAuthHint::Failed(err.clone()),
        // Login is starting (auto-triggered at startup); the URL arrives via AuthUrlReady, which flips us to `Authenticating`
        AuthState::Pending { error: None } => MinimalAuthHint::SigningIn {
            url: None,
            code: None,
        },
        AuthState::Done if has_access && !is_zdr_blocked => {
            if let TrustState::Pending { workspace } = trust {
                MinimalAuthHint::TrustFolder {
                    workspace: workspace.clone(),
                }
            } else {
                MinimalAuthHint::Starting
            }
        }
        AuthState::Done => MinimalAuthHint::Starting,
    }
}

/// Rows the no-agent live region needs for `hint` (before path wrap).
/// Used by the overlay host so the viewport grows enough to show the trust question instead of clipping to the idle prompt height.
pub(super) fn auth_hint_rows(hint: &MinimalAuthHint, width: u16) -> u16 {
    match hint {
        // header + blank + "Opening browser…"
        MinimalAuthHint::SigningIn { url: None, code: _ } => 3,
        // header + blank + "Open this URL" + url rows + optional code block + blank + "Waiting…"
        MinimalAuthHint::SigningIn {
            url: Some(url),
            code,
        } => {
            let url_rows = wrapped_char_rows(url, width);
            let code_rows = if code.is_some() { 2 } else { 0 }; // blank + "Code: …"
            3 + url_rows + code_rows + 2
        }
        // "Sign-in failed" + blank + error
        MinimalAuthHint::Failed(_) => 3,
        // question + path rows + blank + 2 warning + blank + 2 menu + blank + hint
        MinimalAuthHint::TrustFolder { workspace } => {
            let path = workspace.display().to_string();
            let path_rows = wrapped_char_rows(&path, width);
            1 + path_rows + 1 + 2 + 1 + 2 + 1 + 1
        }
        MinimalAuthHint::Starting | MinimalAuthHint::SigningInPassword => 1,
        // title + issuer + blank + 3 labels + 3 values + blank + hint, plus the error line when set
        MinimalAuthHint::Credentials { error, .. } => 11 + u16::from(error.is_some()),
    }
}

fn credential_hint(form: &CredentialForm) -> MinimalAuthHint {
    let (password_mask, password_cursor) =
        masked_chars(form.password.text(), form.password.cursor_byte());
    MinimalAuthHint::Credentials {
        issuer: form.issuer.clone(),
        email: form.email.text().to_owned(),
        email_cursor: char_index(form.email.text(), form.email.cursor_byte()),
        password_mask,
        password_cursor,
        machine_code: form.machine_code.text().to_owned(),
        machine_cursor: char_index(form.machine_code.text(), form.machine_code.cursor_byte()),
        focus: form.focus,
        error: form.error.clone(),
    }
}

fn char_index(text: &str, cursor_byte: usize) -> usize {
    text.get(..cursor_byte.min(text.len()))
        .unwrap_or("")
        .chars()
        .count()
}

fn masked_chars(text: &str, cursor_byte: usize) -> (String, usize) {
    let mut display = String::new();
    let mut cursor = 0usize;
    let mut seen = 0usize;
    for ch in text.chars() {
        if seen < cursor_byte {
            cursor += 1;
        }
        seen += ch.len_utf8();
        display.push('*');
    }
    if cursor_byte >= text.len() {
        cursor = display.chars().count();
    }
    (display, cursor)
}

/// How many rows `text` needs when painted char-by-char at `width` (no wrap-inserted spaces); same layout as [`render_url`].
fn wrapped_char_rows(text: &str, width: u16) -> u16 {
    let width = width.max(1) as usize;
    let chars = text.chars().filter(|c| !c.is_control()).count();
    if chars == 0 {
        return 1;
    }
    chars.div_ceil(width) as u16
}

/// Parse the device-flow `user_code` from a verification URL (`None` if absent or malformed).
/// Mirrors `views::welcome::extract_user_code`, kept local so minimal does not depend on welcome-screen internals.
fn device_user_code(url: &str) -> Option<&str> {
    let code = url
        .split('?')
        .nth(1)?
        .split('&')
        .find_map(|kv| kv.strip_prefix("user_code="))?;
    (!code.is_empty() && code.chars().all(|c| c.is_ascii_alphanumeric() || c == '-'))
        .then_some(code)
}

/// Write `line` at row `y` (when it fits) and return the next row.
fn put_line(buf: &mut Buffer, area: Rect, y: u16, bottom: u16, line: Line<'_>) -> u16 {
    if y < bottom {
        buf.set_line(area.x, y, &line, area.width);
        y + 1
    } else {
        y
    }
}

/// Write `url` char-by-char across as many rows as it needs (no wrap-inserted spaces), so the terminal's native selection copies it verbatim.
/// Minimal has no mouse capture, so copy is the terminal's job.
/// Returns the next free row.
fn render_url(
    buf: &mut Buffer,
    area: Rect,
    start_y: u16,
    bottom: u16,
    url: &str,
    style: Style,
) -> u16 {
    let width = area.width.max(1);
    // Snapshot the buffer bounds as values so the `&Rect` borrow doesn't outlive the mutable cell writes below
    let (max_x, max_y) = {
        let a = buf.area();
        (a.right(), a.bottom())
    };
    let mut col = 0u16;
    let mut y = start_y;
    for ch in url.chars() {
        // Skip control chars to prevent terminal escape injection.
        if ch.is_control() {
            continue;
        }
        if col >= width {
            col = 0;
            y = y.saturating_add(1);
        }
        if y >= bottom {
            return bottom;
        }
        let x = area.x + col;
        if x < max_x
            && y < max_y
            && let Some(cell) = buf.cell_mut((x, y))
        {
            cell.set_char(ch).set_style(style);
        }
        col += 1;
    }
    y.saturating_add(1)
}

/// Render the sign-in / trust flow (or transient status) in the live region when no agent exists yet.
/// Top-aligned in `area`; clips to its height.
pub(super) fn render_auth(buf: &mut Buffer, area: Rect, theme: &Theme, hint: &MinimalAuthHint) {
    if area.width == 0 || area.height == 0 {
        return;
    }
    let bottom = area.y + area.height;
    let mut y = area.y;
    let gray = theme.muted().bg(Color::Reset);
    let bold = Style::default()
        .fg(theme.text_primary)
        .add_modifier(Modifier::BOLD)
        .bg(Color::Reset);

    match hint {
        MinimalAuthHint::SigningIn { url, code } => {
            y = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled("Sign in to Atlas", bold)),
            );
            y = put_line(buf, area, y, bottom, Line::default());
            match url {
                Some(url) => {
                    y = put_line(
                        buf,
                        area,
                        y,
                        bottom,
                        Line::from(Span::styled(
                            "Open this URL in your browser to approve:",
                            gray,
                        )),
                    );
                    y = render_url(
                        buf,
                        area,
                        y,
                        bottom,
                        url,
                        Style::default().fg(theme.accent_user).bg(Color::Reset),
                    );
                    if let Some(code) = code {
                        y = put_line(buf, area, y, bottom, Line::default());
                        y = put_line(
                            buf,
                            area,
                            y,
                            bottom,
                            Line::from(vec![
                                Span::styled("Code: ", gray),
                                Span::styled(code.clone(), bold),
                            ]),
                        );
                    }
                    y = put_line(buf, area, y, bottom, Line::default());
                    let _ = put_line(
                        buf,
                        area,
                        y,
                        bottom,
                        Line::from(Span::styled("Waiting for approval\u{2026}", gray)),
                    );
                }
                None => {
                    let _ = put_line(
                        buf,
                        area,
                        y,
                        bottom,
                        Line::from(Span::styled(
                            "Opening your browser to sign in\u{2026}",
                            gray,
                        )),
                    );
                }
            }
        }
        MinimalAuthHint::Failed(err) => {
            let warn = Style::default()
                .fg(theme.warning)
                .add_modifier(Modifier::BOLD)
                .bg(Color::Reset);
            y = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled("Sign-in failed", warn)),
            );
            y = put_line(buf, area, y, bottom, Line::default());
            let _ = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled(err.clone(), gray)),
            );
        }
        MinimalAuthHint::TrustFolder { workspace } => {
            // Mirrors `render_welcome_trust` copy, flush-left for minimal.
            y = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled(
                    "Do you trust the contents of this directory?",
                    bold,
                )),
            );
            y = render_url(
                buf,
                area,
                y,
                bottom,
                &workspace.display().to_string(),
                Style::default().fg(theme.accent_user).bg(Color::Reset),
            );
            y = put_line(buf, area, y, bottom, Line::default());
            y = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled(
                    "Atlas may run or modify contents in this directory,",
                    gray,
                )),
            );
            y = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled("posing security risks.", gray)),
            );
            y = put_line(buf, area, y, bottom, Line::default());
            y = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(vec![
                    Span::styled("y", bold),
                    Span::styled("  Yes, proceed", gray),
                ]),
            );
            y = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(vec![
                    Span::styled("n", bold),
                    Span::styled("  No, quit", gray),
                ]),
            );
            y = put_line(buf, area, y, bottom, Line::default());
            let _ = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled(
                    "Enter or y to trust \u{00b7} n or Esc to quit",
                    gray,
                )),
            );
        }
        MinimalAuthHint::Starting => {
            let _ = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled(
                    "Signing in\u{2026} starting your session.",
                    gray,
                )),
            );
        }
        MinimalAuthHint::SigningInPassword => {
            let _ = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled("正在登录...", gray)),
            );
        }
        MinimalAuthHint::Credentials {
            issuer,
            email,
            email_cursor,
            password_mask,
            password_cursor,
            machine_code,
            machine_cursor,
            focus,
            error,
        } => {
            y = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled("登录", bold)),
            );
            y = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled(format!("服务器 {issuer}"), gray)),
            );
            y = put_line(buf, area, y, bottom, Line::default());
            y = render_login_row(
                buf,
                area,
                y,
                bottom,
                theme,
                "邮箱",
                email,
                *email_cursor,
                "name@example.com",
                *focus == CredentialField::Email,
            );
            y = render_login_row(
                buf,
                area,
                y,
                bottom,
                theme,
                "密码",
                password_mask,
                *password_cursor,
                "密码",
                *focus == CredentialField::Password,
            );
            y = render_login_row(
                buf,
                area,
                y,
                bottom,
                theme,
                "机器码",
                machine_code,
                *machine_cursor,
                "ABCD-EFGH",
                *focus == CredentialField::MachineCode,
            );
            if let Some(err) = error {
                let warn = Style::default().fg(theme.warning).bg(Color::Reset);
                y = put_line(
                    buf,
                    area,
                    y,
                    bottom,
                    Line::from(Span::styled(err.clone(), warn)),
                );
            }
            y = put_line(buf, area, y, bottom, Line::default());
            let _ = put_line(
                buf,
                area,
                y,
                bottom,
                Line::from(Span::styled("Tab 切换   Enter 登录   Esc 返回", gray)),
            );
        }
    }
}

fn render_login_row(
    buf: &mut Buffer,
    area: Rect,
    mut y: u16,
    bottom: u16,
    theme: &Theme,
    label: &str,
    text: &str,
    cursor: usize,
    placeholder: &str,
    focused: bool,
) -> u16 {
    let gray = theme.muted().bg(Color::Reset);
    let label_style = if focused {
        Style::default()
            .fg(theme.text_primary)
            .add_modifier(Modifier::BOLD)
            .bg(Color::Reset)
    } else {
        gray
    };
    y = put_line(
        buf,
        area,
        y,
        bottom,
        Line::from(Span::styled(label, label_style)),
    );
    put_line(
        buf,
        area,
        y,
        bottom,
        login_value_line(theme, text, cursor, placeholder, focused),
    )
}

fn login_value_line(
    theme: &Theme,
    text: &str,
    cursor: usize,
    placeholder: &str,
    focused: bool,
) -> Line<'static> {
    let text_style = Style::default().fg(theme.text_primary).bg(Color::Reset);
    let dim = Style::default().fg(theme.gray_dim).bg(Color::Reset);
    let marker = if focused { "> " } else { "  " };
    let mut spans = vec![Span::styled(marker, text_style)];
    if text.is_empty() {
        spans.push(Span::styled(placeholder.to_owned(), dim));
        if focused {
            spans.push(Span::styled(
                " ",
                theme.block_cursor_over(theme.bg_base),
            ));
        }
        return Line::from(spans);
    }
    let mut before = String::new();
    let mut at_cursor = None;
    let mut after = String::new();
    for (index, ch) in text.chars().enumerate() {
        if index < cursor {
            before.push(ch);
        } else if at_cursor.is_none() {
            at_cursor = Some(ch);
        } else {
            after.push(ch);
        }
    }
    if !before.is_empty() {
        spans.push(Span::styled(before, text_style));
    }
    if let Some(ch) = at_cursor {
        let styled = if focused {
            theme.block_cursor_over(theme.bg_base)
        } else {
            text_style
        };
        spans.push(Span::styled(ch.to_string(), styled));
        if !after.is_empty() {
            spans.push(Span::styled(after, text_style));
        }
    } else if focused {
        spans.push(Span::styled(" ", theme.block_cursor_over(theme.bg_base)));
    }
    Line::from(spans)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn device_user_code_parses_verification_url() {
        assert_eq!(
            device_user_code("https://accounts.x.ai/oauth2/device?user_code=ABCD-EFGH"),
            Some("ABCD-EFGH")
        );
        assert_eq!(
            device_user_code("https://accounts.x.ai/oauth2/device"),
            None
        );
        assert_eq!(device_user_code("https://x/device?other=1"), None);
    }

    #[test]
    fn auth_hint_maps_auth_state() {
        use xai_grok_pager::app::app_view::AuthMode;

        let trust_done = TrustState::Done;

        // Device flow maps to SigningIn carrying the URL and the parsed code
        let st = AuthState::Authenticating {
            request_seq: 1,
            handle: None,
            auth_url: Some("https://accounts.x.ai/device?user_code=ABCD-EFGH".into()),
            mode: AuthMode::Device,
        };
        match minimal_auth_hint(&st, &trust_done, true, false) {
            MinimalAuthHint::SigningIn { url, code } => {
                assert_eq!(
                    url.as_deref(),
                    Some("https://accounts.x.ai/device?user_code=ABCD-EFGH")
                );
                assert_eq!(code.as_deref(), Some("ABCD-EFGH"));
            }
            _ => panic!("expected SigningIn"),
        }

        // External command flow maps to SigningIn with the URL and no code
        let st = AuthState::Authenticating {
            request_seq: 2,
            handle: None,
            auth_url: Some("https://provider.example/login".into()),
            mode: AuthMode::Command,
        };
        match minimal_auth_hint(&st, &trust_done, true, false) {
            MinimalAuthHint::SigningIn { url, code } => {
                assert_eq!(url.as_deref(), Some("https://provider.example/login"));
                assert!(code.is_none());
            }
            _ => panic!("expected SigningIn"),
        }

        assert!(matches!(
            minimal_auth_hint(&AuthState::Done, &trust_done, true, false),
            MinimalAuthHint::Starting
        ));
        assert!(matches!(
            minimal_auth_hint(
                &AuthState::Pending {
                    error: Some("nope".into())
                },
                &trust_done,
                true,
                false
            ),
            MinimalAuthHint::Failed(_)
        ));
    }

    #[test]
    fn auth_hint_maps_pending_trust_after_auth() {
        let trust = TrustState::Pending {
            workspace: PathBuf::from("/tmp/untrusted-repo"),
        };
        match minimal_auth_hint(&AuthState::Done, &trust, true, false) {
            MinimalAuthHint::TrustFolder { workspace } => {
                assert_eq!(workspace, PathBuf::from("/tmp/untrusted-repo"));
            }
            _ => panic!("expected TrustFolder"),
        }

        // Access / ZDR gates suppress the trust question (matches welcome and the input interceptor)
        assert!(matches!(
            minimal_auth_hint(&AuthState::Done, &trust, false, false),
            MinimalAuthHint::Starting
        ));
        assert!(matches!(
            minimal_auth_hint(&AuthState::Done, &trust, true, true),
            MinimalAuthHint::Starting
        ));

        // Trust is not offered while auth is still in flight.
        assert!(matches!(
            minimal_auth_hint(&AuthState::Pending { error: None }, &trust, true, false),
            MinimalAuthHint::SigningIn { .. }
        ));
    }

    #[test]
    fn render_auth_shows_url_and_code() {
        let theme = Theme::current();
        let area = Rect::new(0, 0, 80, 12);
        let mut buf = Buffer::empty(area);
        let hint = MinimalAuthHint::SigningIn {
            url: Some("https://accounts.x.ai/device?user_code=ABCD-EFGH".into()),
            code: Some("ABCD-EFGH".into()),
        };
        render_auth(&mut buf, area, &theme, &hint);
        let text = crate::buffer_text(&buf);
        assert!(text.contains("Sign in to Atlas"), "header: {text:?}");
        assert!(text.contains("accounts.x.ai/device"), "url: {text:?}");
        assert!(text.contains("ABCD-EFGH"), "device code: {text:?}");
        assert!(
            text.contains("Waiting for approval"),
            "waiting line: {text:?}"
        );
    }

    #[test]
    fn render_auth_shows_trust_question() {
        let theme = Theme::current();
        let area = Rect::new(0, 0, 80, 14);
        let mut buf = Buffer::empty(area);
        let hint = MinimalAuthHint::TrustFolder {
            workspace: PathBuf::from("/home/agent/project"),
        };
        render_auth(&mut buf, area, &theme, &hint);
        let text = crate::buffer_text(&buf);
        assert!(
            text.contains("Do you trust the contents of this directory?"),
            "question: {text:?}"
        );
        assert!(
            text.contains("/home/agent/project"),
            "workspace path: {text:?}"
        );
        assert!(text.contains("Yes, proceed"), "yes option: {text:?}");
        assert!(text.contains("No, quit"), "no option: {text:?}");
        assert!(text.contains("Enter or y to trust"), "hint line: {text:?}");
        assert!(text.contains("posing security risks"), "warning: {text:?}");
    }

    #[test]
    fn auth_hint_rows_covers_trust_path_wrap() {
        let long = "x".repeat(200);
        let hint = MinimalAuthHint::TrustFolder {
            workspace: PathBuf::from(long),
        };
        let rows = auth_hint_rows(&hint, 40);
        // The 200-char path wraps to 5 rows at width 40, so the total sits well above the fixed rows
        assert!(rows >= 12, "expected room for wrapped path, got {rows}");
    }

    #[test]
    fn auth_hint_maps_password_form_without_showing_the_secret() {
        use xai_grok_pager::input::line_editor::LineEditor;

        let mut email = LineEditor::default();
        email.set_text("user@example.com");
        let mut password = LineEditor::default();
        password.set_text("atlas123");
        let mut machine_code = LineEditor::default();
        machine_code.set_text("ABCD-EFGH");
        let form = CredentialForm {
            email,
            password,
            machine_code,
            focus: CredentialField::Password,
            error: Some("邮箱、密码或机器码不正确".into()),
            issuer: "http://atlas.example/atlas".into(),
        };
        let hint = minimal_auth_hint(
            &AuthState::Credentials(Box::new(form)),
            &TrustState::Done,
            true,
            false,
        );
        match &hint {
            MinimalAuthHint::Credentials {
                email,
                password_mask,
                machine_code,
                focus,
                error,
                issuer,
                ..
            } => {
                assert_eq!(email, "user@example.com");
                assert_eq!(password_mask, "********");
                assert!(!password_mask.contains("atlas123"));
                assert_eq!(machine_code, "ABCD-EFGH");
                assert_eq!(*focus, CredentialField::Password);
                assert_eq!(error.as_deref(), Some("邮箱、密码或机器码不正确"));
                assert_eq!(issuer, "http://atlas.example/atlas");
            }
            _ => panic!("expected Credentials"),
        }
        assert_eq!(auth_hint_rows(&hint, 80), 12);

        let signing = AuthState::Authenticating {
            request_seq: 3,
            handle: None,
            auth_url: None,
            mode: AuthMode::Password,
        };
        assert!(matches!(
            minimal_auth_hint(&signing, &TrustState::Done, true, false),
            MinimalAuthHint::SigningInPassword
        ));

        let theme = Theme::current();
        let area = Rect::new(0, 0, 80, 14);
        let mut buf = Buffer::empty(area);
        render_auth(&mut buf, area, &theme, &hint);
        let text = crate::buffer_text(&buf);
        // Wide characters occupy two cells, so the dump reads "登 录".
        assert!(text.contains("登 录"), "title: {text:?}");
        assert!(text.contains("atlas.example"), "issuer: {text:?}");
        assert!(text.contains("user@example.com"), "email: {text:?}");
        assert!(text.contains("********"), "masked password: {text:?}");
        assert!(
            !text.contains("atlas123"),
            "password must stay masked: {text:?}"
        );
        assert!(text.contains("ABCD-EFGH"), "machine code: {text:?}");
        assert!(text.contains("Enter"), "hint: {text:?}");
    }
}
