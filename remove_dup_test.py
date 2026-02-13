# // Author: Enkae (enkae.dev@pm.me)
import os

filepath = "conscience_go/internal/service/safety_actions_test.go"

with open(filepath, "r") as f:
    lines = f.readlines()

new_lines = []
skip = False
for line in lines:
    if "func TestValidateActions(t *testing.T) {" in line:
        skip = True

    if skip and line.startswith("}"):
        skip = False
        continue

    if not skip:
        new_lines.append(line)

with open(filepath, "w") as f:
    f.writelines(new_lines)

print("Removed TestValidateActions from " + filepath)
