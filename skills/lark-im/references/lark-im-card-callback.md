
# Card Callback Workflow

> **Prerequisite:** Read [`../lark-shared/SKILL.md`](../../lark-shared/SKILL.md) first.

Send an interactive card with callback buttons, listen for user interactions via `card.action.trigger`, then update the card in-place.

**Identity / Risk:**
- **Send card**: bot or user — see [lark-im-messages-send](lark-im-messages-send.md) for identity details
- **Listen for callbacks**: bot-only — WebSocket uses App ID + Secret regardless of who sent the card
- **Update card**: same identity as the sender (bot updates bot-sent cards; user updates user-sent cards)

## Platform Configuration (one-time)

In the Lark Open Platform console:
1. **Events & Callbacks → Subscription method** → select "Use long connection to receive events"
2. **Add event**: `card.action.trigger`
3. **Card Callback URL** → leave **empty** (non-empty URL routes callbacks to HTTP, bypassing WebSocket)
4. Publish a new app version for changes to take effect

## Step 1 — Send a Card

See [`lark-im-messages-send`](lark-im-messages-send.md) for the full `+messages-send` reference. For a callback card, two things matter:

- Pass `--msg-type interactive` with a card JSON as `--content`
- Each button must have a `value` object — this is what the callback returns

Save the `message_id` (`om_xxx`) from the response — you'll need it in Step 3.

## Step 2 — Listen for Callbacks

Subscribe to `card.action.trigger`. Each button click emits one NDJSON line to stdout.

```bash
lark-cli event +subscribe \
  --event-types card.action.trigger \
  --compact --quiet --as bot
```

### Compact output fields

| Field | Description |
|-------|-------------|
| `type` | `"card.action.trigger"` |
| `event_id` | Unique event ID (for dedup) |
| `timestamp` | Header create time |
| `operator_id` | `open_id` of the user who clicked |
| `action_tag` | Component type: `"button"`, `"select_static"`, `"input"`, etc. |
| `action_value` | The `value` object you set on the component |
| `action_option` | Selected option (select components) |
| `input_value` | Text entered (input components) |
| `checked` | Boolean state (checkbox components) |
| `message_id` | `open_message_id` of the card (`om_xxx`) |
| `chat_id` | `open_chat_id` of the conversation (`oc_xxx`) |

### Example output

```json
{"type":"card.action.trigger","event_id":"abc123","timestamp":"1775712040217490","operator_id":"ou_xxx","action_tag":"button","action_value":{"choice":"noodles","label":"Noodles"},"message_id":"om_xxx","chat_id":"oc_xxx"}
```

## Step 3 — Update the Card

Use `PATCH /open-apis/im/v1/messages/{message_id}` with `msg_type: "interactive"`.

> **Important**: `content` must be a **JSON-encoded string** (the card JSON serialized as a string, not an inline object).

```bash
# Build updated card JSON
UPDATED_CARD=$(python3 -c "
import json, sys
operator, label = sys.argv[1], sys.argv[2]
card = {
  'config': {'wide_screen_mode': True},
  'header': {
    'title': {'tag': 'plain_text', 'content': '🗳️ Vote received!'},
    'template': 'green'
  },
  'elements': [{
    'tag': 'div',
    'text': {
      'tag': 'lark_md',
      'content': f'**{operator}** voted: **{label}** ✅'
    }
  }]
}
print(json.dumps(card))
" "$OPERATOR_ID" "$LABEL")

# content must be double-encoded (a JSON string containing the card JSON)
CONTENT=$(python3 -c "import sys,json; print(json.dumps(sys.argv[1]))" "$UPDATED_CARD")

lark-cli api PATCH "/open-apis/im/v1/messages/${MESSAGE_ID}" \
  --as bot \
  --data "{\"msg_type\":\"interactive\",\"content\":${CONTENT}}"
```

## Full Pipeline (bash)

```bash
MESSAGE_ID="om_xxx"   # from Step 1

lark-cli event +subscribe \
  --event-types card.action.trigger \
  --compact --quiet --as bot \
  | while IFS= read -r line; do
      msg_id=$(echo "$line" | python3 -c "import sys,json; print(json.load(sys.stdin).get('message_id',''))")
      [[ "$msg_id" != "$MESSAGE_ID" ]] && continue

      operator=$(echo "$line" | python3 -c "import sys,json; print(json.load(sys.stdin).get('operator_id',''))")
      label=$(echo "$line" | python3 -c "import sys,json; v=json.load(sys.stdin).get('action_value',{}); print(v.get('label','?'))")

      UPDATED=$(python3 -c "
import sys,json
card={'config':{'wide_screen_mode':True},'header':{'title':{'tag':'plain_text','content':'✅ Done'},'template':'green'},'elements':[{'tag':'div','text':{'tag':'lark_md','content':f'**{sys.argv[1]}** chose **{sys.argv[2]}**'}}]}
print(json.dumps(card))
" "$operator" "$label")

      CONTENT=$(python3 -c "import sys,json; print(json.dumps(sys.argv[1]))" "$UPDATED")
      lark-cli api PATCH "/open-apis/im/v1/messages/${MESSAGE_ID}" \
        --as bot \
        --data "{\"msg_type\":\"interactive\",\"content\":${CONTENT}}"
      break
    done
```

## Full Pipeline (Python — recommended for robustness)

```python
#!/usr/bin/env python3
"""Listen for card.action.trigger and update the card on click."""
import subprocess, sys, json

MESSAGE_ID = "om_xxx"   # target card message_id

proc = subprocess.Popen(
    ["lark-cli", "event", "+subscribe",
     "--event-types", "card.action.trigger",
     "--compact", "--quiet", "--as", "bot"],
    stdout=subprocess.PIPE, stderr=sys.stderr, text=True
)

for line in proc.stdout:
    line = line.strip()
    if not line:
        continue
    try:
        d = json.loads(line)
    except Exception:
        continue

    if d.get("message_id") != MESSAGE_ID:
        continue

    operator = d.get("operator_id", "unknown")
    label    = (d.get("action_value") or {}).get("label", "?")

    card = {
        "config": {"wide_screen_mode": True},
        "header": {"title": {"tag": "plain_text", "content": "✅ Vote received!"}, "template": "green"},
        "elements": [{"tag": "div", "text": {"tag": "lark_md", "content": f"**{operator}** chose **{label}**"}}]
    }
    body = json.dumps({"msg_type": "interactive", "content": json.dumps(card)})
    result = subprocess.run(
        ["lark-cli", "api", "PATCH", f"/open-apis/im/v1/messages/{MESSAGE_ID}",
         "--as", "bot", "--data", body],
        capture_output=True, text=True
    )
    print(result.stdout.strip())
    proc.terminate()
    break

proc.wait()
```

## Permissions

| Operation | Identity | Required scope |
|-----------|----------|---------------|
| Send card | bot | `im:message:send_as_bot` |
| Send card | user | `im:message` |
| Receive card callback | bot (always) | `im:message:receive_as_bot` |
| Update card | bot | `im:message` |
| Update card | user | `im:message` |

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| WebSocket connects but no events | `card.action.trigger` not subscribed in console, or app not published after adding it | Add event in console → publish new version |
| Events arrive via HTTP instead of WebSocket | "Card Callback URL" is set in console | Clear the URL field in console |
| `content` field error on PATCH | Card JSON passed as object, not string | Double-encode: `json.dumps(json.dumps(card))` |
| `--force` needed on `+subscribe` | Another subscribe process already holds the lock | Kill old process or use `--force` |
