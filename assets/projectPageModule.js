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
            project: `${env.APP_URL}/projects/${context.entity_data.ID}`,
        }

        const url = urls[context.entity_type]

        if (!url) {
            console.error('Error: cannot delete entity ' + context.entity_type)
            return
        }

        axios.delete(url)
            .then(resp => {
                window.location.href = `${env.APP_URL}/projects`
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

    // Delete Features
    // const triggers = document.querySelectorAll('[data-issuez-modal-trigger="feature"]')


    // const $modal = $('[data-issuez-delete-modal="feature"]')

    // let targetFeature = null
    // let deleteConfirmBtn = null

    // $modal.on('hidden.bs.modal', function (e) {
        // deleteConfirmBtn.removeEventListener('click', confirmDelete)
    // })

    // $modal.on('show.bs.modal', function (e) {
        // deleteConfirmBtn = document.querySelector('[data-feature-delete-confirm]')

        // const content = document.querySelector('[data-delete-modal-content]')
        // const title = document.querySelector('[data-delete-modal-title]')

        // title.innerHTML = "Delete " + (targetFeature.Name || '')

        // content.innerHTML = 'Are you sure?'

        // deleteConfirmBtn.addEventListener('click', confirmDelete)
    // })

    // triggers.forEach(trigger => {
        // trigger.addEventListener('click', evt => {

            // const feature = evt.currentTarget

            // const featureDataRaw = feature.getAttribute('data-feature')

            // targetFeature = featureDataRaw
                // ? JSON.parse(featureDataRaw)
                // : {}

            // $modal.modal('show')
        // })
    // })

    // function confirmDelete() {

        // axios.delete(`${env.APP_URL}/features/${targetFeature.ID}`)
            // .then(resp => {
                // window.location.href = `${env.APP_URL}/projects/${targetFeature.ProjectID}`
            // })
            // .catch(err => {
                // console.log(err)
            // })
    // }
}
