// Import page dependencies
import * as $ from 'jquery';
import 'jquery-ui/ui/widgets/sortable';
import 'bootstrap/js/src/util';
import 'bootstrap/js/src/tab';
import 'bootstrap/js/src/collapse';
import 'bootstrap/js/src/dropdown';

import * as L from 'leaflet';

import set_emoji_favicon from './emoji-favicon-toolkit';

function humanFileSize(bytes: number, si: boolean): string {
    const thresh = si ? 1000 : 1024;
    if (Math.abs(bytes) < thresh) {
        return bytes + ' B';
    }
    const units = si
        ? ['KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']
        : ['KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB', 'ZiB', 'YiB'];
    let u = -1;
    do {
        bytes /= thresh;
        ++u;
    } while (Math.abs(bytes) >= thresh && u < units.length - 1);
    return bytes.toFixed(1) + ' ' + units[u];
}

$(() => {
    // Check for a "Generate UUID" button on the page.
    $('.zerodrop-uuid').click((event: JQuery.Event) => {
        const element = $(event.currentTarget);
        $(element.data('field')).val('xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
            const r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        }));
    });

    // Custom file upload input
    $('.zerodrop-file').change((event: JQuery.Event) => {
        const element = $(event.currentTarget);
        const file = (<FileList>element.prop('files'))[0];
        $(element.data('name')).text(`${file.name} (${humanFileSize(file.size, true)})`);
        $(element.data('mime')).val(file.type);
    });

    // New entry tabs
    $('.zerodrop-nav').click((event: JQuery.Event) => {
        const element = $(event.currentTarget);
        element.find('input').prop('checked', true);
    });

    ($('.zerodrop-blacklist-items') as any).sortable({
        handle: '.card-header'
    });

    const map = $('.zerodrop-map').get(0);
    const leaflet = L.map(map, {
        center: [51.505, -0.09],
        zoom: 2
    });

    L.tileLayer('https://api.mapbox.com/styles/v1/dsamarin/cjhod9wkf003m2souy4fltiou/tiles/256/{z}/{x}/{y}?access_token={accessToken}', {
        attribution: 'Map data &copy; <a href="https://www.openstreetmap.org/">OpenStreetMap</a> contributors, <a href="https://creativecommons.org/licenses/by-sa/2.0/">CC-BY-SA</a>, Imagery © <a href="https://www.mapbox.com/">Mapbox</a>',
        maxZoom: 18,
        accessToken: 'pk.eyJ1IjoiZHNhbWFyaW4iLCJhIjoiY2pob2Q4cDF6MTNlZDNkcnltb2trNnpwbCJ9.UUx6bmFfKjYV2w1x61q9nw'
    }).addTo(leaflet);

});

const emojis = ['\u26a0\ufe0f', '\uD83D\uDD17']
let index = 0;
function update_emoji() {
    const emoji = emojis[index];
    index = (index + 1) % emojis.length;
    set_emoji_favicon(emoji);
}
update_emoji();
setInterval(update_emoji, 1337)
