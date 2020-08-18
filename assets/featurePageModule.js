import env from './env'
import axios from 'axios'

export default () => {
    const delete_triggers = document.querySelectorAll('[data-delete-trigger]')

    const $modal = $('[data-issuez-delete-modal]')

    let deleteConfirmBtn = null
    let context = {
        entity_type: null,
        entity_data: null,
    }

    delete_triggers.forEach(trigger => {
        trigger.addEventListener('click', evt => {

            // set context
            const triggerEl = evt.currentTarget

            context.entity_type = triggerEl.getAttribute('data-delete-trigger')
            const entityRaw = triggerEl.getAttribute('data-entity')

            context.entity_data = entityRaw
                ? JSON.parse(entityRaw)
                : {}

            $modal.modal('show')
        })
    })

    function confirmDelete() {

        const urls = {
            feature: `${env.APP_URL}/features/${context.entity_data.ID}`,
            story: `${env.APP_URL}/stories/${context.entity_data.ID}`,
            bug: `${env.APP_URL}/bugs/${context.entity_data.ID}`,
        }

        const url = urls[context.entity_type]

        if (!url) {
            console.error('Error: cannot delete entity ' + context.entity_type)
            return
        }

        axios.delete(url)
            .then(resp => {
                const feature_id = context.entity_data.FeatureID || context.entity_data.ID

                window.location.href = `${env.APP_URL}/features/${feature_id}`
            })
            .catch(err => {
                alert(err.message)
                console.log(err)
            })
    }

    // Modal Events
    $modal.on('hidden.bs.modal', function (e) {
        deleteConfirmBtn.removeEventListener('click', confirmDelete)
    })

    $modal.on('show.bs.modal', function (e) {
        // update modal content
        const content = document.querySelector('[data-delete-modal-content]')
        const title = document.querySelector('[data-delete-modal-title]')

        title.innerHTML = "Delete " + (context.entity_data.Name || '')

        content.innerHTML = 'Are you sure?'

        // delete confirmation
        deleteConfirmBtn = document.querySelector('[data-delete-confirm]')
        deleteConfirmBtn.addEventListener('click', confirmDelete)
    })
}
