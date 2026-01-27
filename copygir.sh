#!/bin/sh

set -e

# GIR files not available in org.gnome.Sdk (e.g., EDS libraries)
SYSTEM_ONLY_GIRS="Camel-1.2.gir EBook-1.2.gir EBookContacts-1.2.gir ECal-2.0.gir EDataServer-1.2.gir ICalGLib-3.0.gir"

for f in internal/gir/spec/*.gir; do
    basename_f=$(basename "${f}")

    # Check if this GIR file should be copied from system instead of Flatpak SDK
    if echo "${SYSTEM_ONLY_GIRS}" | grep -qw "${basename_f}"; then
        echo "Copying ${basename_f} from system..."
        cp "/usr/share/gir-1.0/${basename_f}" "${f}"
    else
        flatpak run --filesystem="${PWD}" --command=sh org.gnome.Sdk -c "cp /usr/share/gir-1.0/${basename_f} ${PWD}/${f}"
    fi
done
