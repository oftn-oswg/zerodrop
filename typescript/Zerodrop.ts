// Import page dependencies
import * as $ from 'jquery';
import 'bootstrap/js/dist/util'
import 'bootstrap/js/dist/dropdown'
import 'bootstrap/js/dist/tab'

// Check for a "Generate UUID" button on the page.
$('.zerodrop-uuid').click(() => {
    $($(this).data('field')).val('xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
        var r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    }));
});

// Custom file upload input
$('.zerodrop-file').change(() => {
    const file = (<FileList>$(this).prop('files'))[0];
    $($(this).data('filename')).text(file.name);
    $($(this).data('filemime')).val(file.type);
});

// New entry tabs
$('.zerodrop-nav').click(() => {
    $(this).find('input').prop('checked', true);
});
