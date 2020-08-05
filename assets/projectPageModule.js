import env from './env'
import axios from 'axios'

export default () => {
    const triggers = document.querySelectorAll('[data-issuez-modal-trigger="feature"]')

    const $modal = $('[data-issuez-delete-modal="feature"]')

    let targetFeature = null
    let deleteConfirmBtn = null

    $modal.on('hidden.bs.modal', function (e) {
        deleteConfirmBtn.removeEventListener('click', confirmDelete)
    })

    $modal.on('show.bs.modal', function (e) {
        deleteConfirmBtn = document.querySelector('[data-feature-delete-confirm]')

        const content = document.querySelector('[data-delete-modal-content]')
        const title = document.querySelector('[data-delete-modal-title]')

        title.innerHTML = "Delete " + (targetFeature.Name || '')

        content.innerHTML = 'Are you sure?'

        deleteConfirmBtn.addEventListener('click', confirmDelete)
    })

    triggers.forEach(trigger => {
        trigger.addEventListener('click', evt => {

            const feature = evt.currentTarget

            const featureDataRaw = feature.getAttribute('data-feature')

            targetFeature = featureDataRaw
                ? JSON.parse(featureDataRaw)
                : {}

            $modal.modal('show')
        })
    })

    function confirmDelete() {

        axios.delete(`${env.APP_URL}/features/${targetFeature.ID}`)
            .then(resp => {
                window.location.href = `${env.APP_URL}/projects/${targetFeature.ProjectID}`
            })
            .catch(err => {
                console.log(err)
            })
    }
}
